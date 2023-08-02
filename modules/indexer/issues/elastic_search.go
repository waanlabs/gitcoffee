// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"

	"github.com/olivere/elastic/v7"
)

var _ Indexer = &ElasticSearchIndexer{}

// ElasticSearchIndexer implements Indexer interface
type ElasticSearchIndexer struct {
	client      *elastic.Client
	indexerName string
	available   bool
	stopTimer   chan struct{}
	lock        sync.RWMutex
}

// NewElasticSearchIndexer creates a new elasticsearch indexer
func NewElasticSearchIndexer(url, indexerName string) (*ElasticSearchIndexer, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetGzip(false),
	}

	logger := log.GetLogger(log.DEFAULT)
	opts = append(opts, elastic.SetTraceLog(&log.PrintfLogger{Logf: logger.Trace}))
	opts = append(opts, elastic.SetInfoLog(&log.PrintfLogger{Logf: logger.Info}))
	opts = append(opts, elastic.SetErrorLog(&log.PrintfLogger{Logf: logger.Error}))

	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	indexer := &ElasticSearchIndexer{
		client:      client,
		indexerName: indexerName,
		available:   true,
		stopTimer:   make(chan struct{}),
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				indexer.checkAvailability()
			case <-indexer.stopTimer:
				ticker.Stop()
				return
			}
		}
	}()

	return indexer, nil
}

const (
	defaultMapping = `{
		"mappings": {
			"properties": {
				"id": {
					"type": "integer",
					"index": true
				},
				"repo_id": {
					"type": "integer",
					"index": true
				},
				"title": {
					"type": "text",
					"index": true
				},
				"content": {
					"type": "text",
					"index": true
				},
				"comments": {
					"type" : "text",
					"index": true
				}
			}
		}
	}`
)

// Init will initialize the indexer
func (b *ElasticSearchIndexer) Init() (bool, error) {
	ctx := graceful.GetManager().HammerContext()
	exists, err := b.client.IndexExists(b.indexerName).Do(ctx)
	if err != nil {
		return false, b.checkError(err)
	}

	if !exists {
		mapping := defaultMapping

		createIndex, err := b.client.CreateIndex(b.indexerName).BodyString(mapping).Do(ctx)
		if err != nil {
			return false, b.checkError(err)
		}
		if !createIndex.Acknowledged {
			return false, errors.New("init failed")
		}

		return false, nil
	}
	return true, nil
}

// Ping checks if elastic is available
func (b *ElasticSearchIndexer) Ping() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.available
}

// Index will save the index data
func (b *ElasticSearchIndexer) Index(issues []*IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]
		_, err := b.client.Index().
			Index(b.indexerName).
			Id(fmt.Sprintf("%d", issue.ID)).
			BodyJson(map[string]any{
				"id":       issue.ID,
				"repo_id":  issue.RepoID,
				"title":    issue.Title,
				"content":  issue.Content,
				"comments": issue.Comments,
			}).
			Do(graceful.GetManager().HammerContext())
		return b.checkError(err)
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, issue := range issues {
		reqs = append(reqs,
			elastic.NewBulkIndexRequest().
				Index(b.indexerName).
				Id(fmt.Sprintf("%d", issue.ID)).
				Doc(map[string]any{
					"id":       issue.ID,
					"repo_id":  issue.RepoID,
					"title":    issue.Title,
					"content":  issue.Content,
					"comments": issue.Comments,
				}),
		)
	}

	_, err := b.client.Bulk().
		Index(b.indexerName).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return b.checkError(err)
}

// Delete deletes indexes by ids
func (b *ElasticSearchIndexer) Delete(ids ...int64) error {
	if len(ids) == 0 {
		return nil
	} else if len(ids) == 1 {
		_, err := b.client.Delete().
			Index(b.indexerName).
			Id(fmt.Sprintf("%d", ids[0])).
			Do(graceful.GetManager().HammerContext())
		return b.checkError(err)
	}

	reqs := make([]elastic.BulkableRequest, 0)
	for _, id := range ids {
		reqs = append(reqs,
			elastic.NewBulkDeleteRequest().
				Index(b.indexerName).
				Id(fmt.Sprintf("%d", id)),
		)
	}

	_, err := b.client.Bulk().
		Index(b.indexerName).
		Add(reqs...).
		Do(graceful.GetManager().HammerContext())
	return b.checkError(err)
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *ElasticSearchIndexer) Search(ctx context.Context, keyword string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	kwQuery := elastic.NewMultiMatchQuery(keyword, "title", "content", "comments")
	query := elastic.NewBoolQuery()
	query = query.Must(kwQuery)
	if len(repoIDs) > 0 {
		repoStrs := make([]any, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoStrs = append(repoStrs, repoID)
		}
		repoQuery := elastic.NewTermsQuery("repo_id", repoStrs...)
		query = query.Must(repoQuery)
	}
	searchResult, err := b.client.Search().
		Index(b.indexerName).
		Query(query).
		Sort("_score", false).
		From(start).Size(limit).
		Do(ctx)
	if err != nil {
		return nil, b.checkError(err)
	}

	hits := make([]Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		id, _ := strconv.ParseInt(hit.Id, 10, 64)
		hits = append(hits, Match{
			ID: id,
		})
	}

	return &SearchResult{
		Total: searchResult.TotalHits(),
		Hits:  hits,
	}, nil
}

// Close implements indexer
func (b *ElasticSearchIndexer) Close() {
	select {
	case <-b.stopTimer:
	default:
		close(b.stopTimer)
	}
}

func (b *ElasticSearchIndexer) checkError(err error) error {
	var opErr *net.OpError
	if !(elastic.IsConnErr(err) || (errors.As(err, &opErr) && (opErr.Op == "dial" || opErr.Op == "read"))) {
		return err
	}

	b.setAvailability(false)

	return err
}

func (b *ElasticSearchIndexer) checkAvailability() {
	if b.Ping() {
		return
	}

	// Request cluster state to check if elastic is available again
	_, err := b.client.ClusterState().Do(graceful.GetManager().ShutdownContext())
	if err != nil {
		b.setAvailability(false)
		return
	}

	b.setAvailability(true)
}

func (b *ElasticSearchIndexer) setAvailability(available bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.available == available {
		return
	}

	b.available = available
}
