// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

// GitHub Actions Artifacts API Simple Description
//
// 1. Upload artifact
// 1.1. Post upload url
// Post: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts?api-version=6.0-preview
// Request:
// {
//  "Type": "actions_storage",
//  "Name": "artifact"
// }
// Response:
// {
// 	"fileContainerResourceUrl":"/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload"
// }
// it acquires an upload url for artifact upload
// 1.2. Upload artifact
// PUT: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload?itemPath=artifact%2Ffilename
// it upload chunk with headers:
//    x-tfs-filelength: 1024 					// total file length
//    content-length: 1024 						// chunk length
//    x-actions-results-md5: md5sum 	// md5sum of chunk
//    content-range: bytes 0-1023/1024 // chunk range
// we save all chunks to one storage directory after md5sum check
// 1.3. Confirm upload
// PATCH: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/upload?itemPath=artifact%2Ffilename
// it confirm upload and merge all chunks to one file, save this file to storage
//
// 2. Download artifact
// 2.1 list artifacts
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts?api-version=6.0-preview
// Response:
// {
// 	"count": 1,
// 	"value": [
// 		{
// 			"name": "artifact",
// 			"fileContainerResourceUrl": "/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/path"
// 		}
// 	]
// }
// 2.2 download artifact
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/path?api-version=6.0-preview
// Response:
// {
//   "value": [
// 			{
// 	 			"contentLocation": "/api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/download",
// 				"path": "artifact/filename",
// 				"itemType": "file"
// 			}
//   ]
// }
// 2.3 download artifact file
// GET: /api/actions_pipeline/_apis/pipelines/workflows/{run_id}/artifacts/{artifact_id}/download?itemPath=artifact%2Ffilename
// Response:
// download file
//

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	web_types "code.gitea.io/gitea/modules/web/types"
)

const artifactRouteBase = "/_apis/pipelines/workflows/{run_id}/artifacts"

type artifactContextKeyType struct{}

var artifactContextKey = artifactContextKeyType{}

type ArtifactContext struct {
	*context.Base

	ActionTask *actions.ActionTask
}

func init() {
	web.RegisterResponseStatusProvider[*ArtifactContext](func(req *http.Request) web_types.ResponseStatusProvider {
		return req.Context().Value(artifactContextKey).(*ArtifactContext)
	})
}

func ArtifactsRoutes(prefix string) *web.Route {
	m := web.NewRoute()
	m.Use(ArtifactContexter())

	r := artifactRoutes{
		prefix: prefix,
		fs:     storage.ActionsArtifacts,
	}

	m.Group(artifactRouteBase, func() {
		// retrieve, list and confirm artifacts
		m.Combo("").Get(r.listArtifacts).Post(r.getUploadArtifactURL).Patch(r.comfirmUploadArtifact)
		// handle container artifacts list and download
		m.Put("/{artifact_hash}/upload", r.uploadArtifact)
		// handle artifacts download
		m.Get("/{artifact_hash}/download_url", r.getDownloadArtifactURL)
		m.Get("/{artifact_id}/download", r.downloadArtifact)
	})

	return m
}

func ArtifactContexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := context.NewBaseContext(resp, req)
			defer baseCleanUp()

			ctx := &ArtifactContext{Base: base}
			ctx.AppendContextValue(artifactContextKey, ctx)

			// action task call server api with Bearer ACTIONS_RUNTIME_TOKEN
			// we should verify the ACTIONS_RUNTIME_TOKEN
			authHeader := req.Header.Get("Authorization")
			if len(authHeader) == 0 || !strings.HasPrefix(authHeader, "Bearer ") {
				ctx.Error(http.StatusUnauthorized, "Bad authorization header")
				return
			}

			authToken := strings.TrimPrefix(authHeader, "Bearer ")
			task, err := actions.GetRunningTaskByToken(req.Context(), authToken)
			if err != nil {
				log.Error("Error runner api getting task: %v", err)
				ctx.Error(http.StatusInternalServerError, "Error runner api getting task")
				return
			}

			if err := task.LoadJob(req.Context()); err != nil {
				log.Error("Error runner api getting job: %v", err)
				ctx.Error(http.StatusInternalServerError, "Error runner api getting job")
				return
			}

			ctx.ActionTask = task
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

type artifactRoutes struct {
	prefix string
	fs     storage.ObjectStorage
}

func (ar artifactRoutes) buildArtifactURL(runID int64, artifactHash, suffix string) string {
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ar.prefix, "/") +
		strings.ReplaceAll(artifactRouteBase, "{run_id}", strconv.FormatInt(runID, 10)) +
		"/" + artifactHash + "/" + suffix
	return uploadURL
}

type getUploadArtifactRequest struct {
	Type string
	Name string
}

type getUploadArtifactResponse struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *ArtifactContext) {
	_, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}

	var req getUploadArtifactRequest
	if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err != nil {
		log.Error("Error decode request body: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error decode request body")
		return
	}

	// use md5(artifact_name) to create upload url
	artifactHash := fmt.Sprintf("%x", md5.Sum([]byte(req.Name)))
	resp := getUploadArtifactResponse{
		FileContainerResourceURL: ar.buildArtifactURL(runID, artifactHash, "upload"),
	}
	log.Debug("[artifact] get upload url: %s", resp.FileContainerResourceURL)
	ctx.JSON(http.StatusOK, resp)
}

func (ar artifactRoutes) uploadArtifact(ctx *ArtifactContext) {
	task, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}
	artifactName, artifactPath, ok := parseArtifactItemPath(ctx)
	if !ok {
		return
	}

	// get upload file size
	fileRealTotalSize, contentLength, err := getUploadFileSize(ctx)
	if err != nil {
		log.Error("Error get upload file size: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error get upload file size")
		return
	}

	// create or get artifact with name and path
	artifact, err := actions.CreateArtifact(ctx, task, artifactName, artifactPath)
	if err != nil {
		log.Error("Error create or get artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error create or get artifact")
		return
	}

	// save chunk to storage, if success, return chunk stotal size
	// if artifact is not gzip when uploading, chunksTotalSize ==  fileRealTotalSize
	// if artifact is gzip when uploading, chunksTotalSize <  fileRealTotalSize
	chunksTotalSize, err := saveUploadChunk(ar.fs, ctx, artifact, contentLength, runID)
	if err != nil {
		log.Error("Error save upload chunk: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error save upload chunk")
		return
	}

	// update artifact size if zero
	if artifact.FileSize == 0 || artifact.FileCompressedSize == 0 {
		artifact.FileSize = fileRealTotalSize
		artifact.FileCompressedSize = chunksTotalSize
		artifact.ContentEncoding = ctx.Req.Header.Get("Content-Encoding")
		if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
			log.Error("Error update artifact: %v", err)
			ctx.Error(http.StatusInternalServerError, "Error update artifact")
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"message": "success",
	})
}

// comfirmUploadArtifact comfirm upload artifact.
// if all chunks are uploaded, merge them to one file.
func (ar artifactRoutes) comfirmUploadArtifact(ctx *ArtifactContext) {
	_, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}
	artifactName := ctx.Req.URL.Query().Get("artifactName")
	if artifactName == "" {
		log.Error("Error artifact name is empty")
		ctx.Error(http.StatusBadRequest, "Error artifact name is empty")
		return
	}
	if err := mergeChunksForRun(ctx, ar.fs, runID, artifactName); err != nil {
		log.Error("Error merge chunks: %v", err)
		ctx.Error(http.StatusInternalServerError, "Error merge chunks")
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{
		"message": "success",
	})
}

type (
	listArtifactsResponse struct {
		Count int64                       `json:"count"`
		Value []listArtifactsResponseItem `json:"value"`
	}
	listArtifactsResponseItem struct {
		Name                     string `json:"name"`
		FileContainerResourceURL string `json:"fileContainerResourceUrl"`
	}
)

func (ar artifactRoutes) listArtifacts(ctx *ArtifactContext) {
	_, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}

	artifacts, err := actions.ListArtifactsByRunID(ctx, runID)
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if len(artifacts) == 0 {
		log.Debug("[artifact] handleListArtifacts, no artifacts")
		ctx.Error(http.StatusNotFound)
		return
	}

	var (
		items  []listArtifactsResponseItem
		values = make(map[string]bool)
	)

	for _, art := range artifacts {
		if values[art.ArtifactName] {
			continue
		}
		artifactHash := fmt.Sprintf("%x", md5.Sum([]byte(art.ArtifactName)))
		item := listArtifactsResponseItem{
			Name:                     art.ArtifactName,
			FileContainerResourceURL: ar.buildArtifactURL(runID, artifactHash, "download_url"),
		}
		items = append(items, item)
		values[art.ArtifactName] = true

		log.Debug("[artifact] handleListArtifacts, name: %s, url: %s", item.Name, item.FileContainerResourceURL)
	}

	respData := listArtifactsResponse{
		Count: int64(len(items)),
		Value: items,
	}
	ctx.JSON(http.StatusOK, respData)
}

type (
	downloadArtifactResponse struct {
		Value []downloadArtifactResponseItem `json:"value"`
	}
	downloadArtifactResponseItem struct {
		Path            string `json:"path"`
		ItemType        string `json:"itemType"`
		ContentLocation string `json:"contentLocation"`
	}
)

// getDownloadArtifactURL generates download url for each artifact
func (ar artifactRoutes) getDownloadArtifactURL(ctx *ArtifactContext) {
	_, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}

	itemPath := util.PathJoinRel(ctx.Req.URL.Query().Get("itemPath"))
	if !validateArtifactHash(ctx, itemPath) {
		return
	}

	artifacts, err := actions.ListArtifactsByRunIDAndArtifactName(ctx, runID, itemPath)
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if len(artifacts) == 0 {
		log.Debug("[artifact] getDownloadArtifactURL, no artifacts")
		ctx.Error(http.StatusNotFound)
		return
	}

	if itemPath != artifacts[0].ArtifactName {
		log.Error("Error dismatch artifact name, itemPath: %v, artifact: %v", itemPath, artifacts[0].ArtifactName)
		ctx.Error(http.StatusBadRequest, "Error dismatch artifact name")
		return
	}

	var items []downloadArtifactResponseItem
	for _, artifact := range artifacts {
		downloadURL := ar.buildArtifactURL(runID, strconv.FormatInt(artifact.ID, 10), "download")
		item := downloadArtifactResponseItem{
			Path:            util.PathJoinRel(itemPath, artifact.ArtifactPath),
			ItemType:        "file",
			ContentLocation: downloadURL,
		}
		log.Debug("[artifact] getDownloadArtifactURL, path: %s, url: %s", item.Path, item.ContentLocation)
		items = append(items, item)
	}
	respData := downloadArtifactResponse{
		Value: items,
	}
	ctx.JSON(http.StatusOK, respData)
}

// downloadArtifact downloads artifact content
func (ar artifactRoutes) downloadArtifact(ctx *ArtifactContext) {
	_, runID, ok := validateRunID(ctx)
	if !ok {
		return
	}

	artifactID := ctx.ParamsInt64("artifact_id")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if errors.Is(err, util.ErrNotExist) {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if artifact.RunID != runID {
		log.Error("Error dismatch runID and artifactID, task: %v, artifact: %v", runID, artifactID)
		ctx.Error(http.StatusBadRequest, err.Error())
		return
	}

	fd, err := ar.fs.Open(artifact.StoragePath)
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer fd.Close()

	// if artifact is compressed, set content-encoding header to gzip
	if artifact.ContentEncoding == "gzip" {
		ctx.Resp.Header().Set("Content-Encoding", "gzip")
	}
	log.Debug("[artifact] downloadArtifact, name: %s, path: %s, storage: %s, size: %d", artifact.ArtifactName, artifact.ArtifactPath, artifact.StoragePath, artifact.FileSize)
	ctx.ServeContent(fd, &context.ServeHeaderOptions{
		Filename:     artifact.ArtifactName,
		LastModified: artifact.CreatedUnix.AsLocalTime(),
	})
}
