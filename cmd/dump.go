// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/go-chi/session"
	"github.com/mholt/archiver/v3"
	"github.com/urfave/cli/v2"
)

func addReader(w archiver.Writer, r io.ReadCloser, info os.FileInfo, customName string, verbose bool) error {
	if verbose {
		log.Info("Adding file %s", customName)
	}

	return w.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   info,
			CustomName: customName,
		},
		ReadCloser: r,
	})
}

func addFile(w archiver.Writer, filePath, absPath string, verbose bool) error {
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	return addReader(w, file, fileInfo, filePath, verbose)
}

func isSubdir(upper, lower string) (bool, error) {
	if relPath, err := filepath.Rel(upper, lower); err != nil {
		return false, err
	} else if relPath == "." || !strings.HasPrefix(relPath, ".") {
		return true, nil
	}
	return false, nil
}

type outputType struct {
	Enum     []string
	Default  string
	selected string
}

func (o outputType) Join() string {
	return strings.Join(o.Enum, ", ")
}

func (o *outputType) Set(value string) error {
	for _, enum := range o.Enum {
		if enum == value {
			o.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", o.Join())
}

func (o outputType) String() string {
	if o.selected == "" {
		return o.Default
	}
	return o.selected
}

var outputTypeEnum = &outputType{
	Enum:    []string{"zip", "tar", "tar.sz", "tar.gz", "tar.xz", "tar.bz2", "tar.br", "tar.lz4", "tar.zst"},
	Default: "zip",
}

// CmdDump represents the available dump sub-command.
var CmdDump = &cli.Command{
	Name:  "dump",
	Usage: "Dump Gitea files and database",
	Description: `Dump compresses all related files and database into zip file.
It can be used for backup and capture Gitea server image to send to maintainer`,
	Action: runDump,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Value:   fmt.Sprintf("gitea-dump-%d.zip", time.Now().Unix()),
			Usage:   "Name of the dump file which will be created. Supply '-' for stdout. See type for available types.",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"V"},
			Usage:   "Show process details",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only display warnings and errors",
		},
		&cli.StringFlag{
			Name:    "tempdir",
			Aliases: []string{"t"},
			Value:   os.TempDir(),
			Usage:   "Temporary dir path",
		},
		&cli.StringFlag{
			Name:    "database",
			Aliases: []string{"d"},
			Usage:   "Specify the database SQL syntax",
		},
		&cli.BoolFlag{
			Name:    "skip-repository",
			Aliases: []string{"R"},
			Usage:   "Skip the repository dumping",
		},
		&cli.BoolFlag{
			Name:    "skip-log",
			Aliases: []string{"L"},
			Usage:   "Skip the log dumping",
		},
		&cli.BoolFlag{
			Name:  "skip-custom-dir",
			Usage: "Skip custom directory",
		},
		&cli.BoolFlag{
			Name:  "skip-lfs-data",
			Usage: "Skip LFS data",
		},
		&cli.BoolFlag{
			Name:  "skip-attachment-data",
			Usage: "Skip attachment data",
		},
		&cli.BoolFlag{
			Name:  "skip-package-data",
			Usage: "Skip package data",
		},
		&cli.BoolFlag{
			Name:  "skip-index",
			Usage: "Skip bleve index data",
		},
		&cli.GenericFlag{
			Name:  "type",
			Value: outputTypeEnum,
			Usage: fmt.Sprintf("Dump output format: %s", outputTypeEnum.Join()),
		},
	},
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	log.Fatal(format, args...)
}

func runDump(ctx *cli.Context) error {
	var file *os.File
	fileName := ctx.String("file")
	outType := ctx.String("type")
	if fileName == "-" {
		file = os.Stdout
		setupConsoleLogger(log.FATAL, log.CanColorStderr, os.Stderr)
	} else {
		for _, suffix := range outputTypeEnum.Enum {
			if strings.HasSuffix(fileName, "."+suffix) {
				fileName = strings.TrimSuffix(fileName, "."+suffix)
				break
			}
		}
		fileName += "." + outType
	}
	setting.MustInstalled()

	// make sure we are logging to the console no matter what the configuration tells us do to
	// FIXME: don't use CfgProvider directly
	if _, err := setting.CfgProvider.Section("log").NewKey("MODE", "console"); err != nil {
		fatal("Setting logging mode to console failed: %v", err)
	}
	if _, err := setting.CfgProvider.Section("log.console").NewKey("STDERR", "true"); err != nil {
		fatal("Setting console logger to stderr failed: %v", err)
	}

	// Set loglevel to Warn if quiet-mode is requested
	if ctx.Bool("quiet") {
		if _, err := setting.CfgProvider.Section("log.console").NewKey("LEVEL", "Warn"); err != nil {
			fatal("Setting console log-level failed: %v", err)
		}
	}

	if !setting.InstallLock {
		log.Error("Is '%s' really the right config path?\n", setting.CustomConf)
		return fmt.Errorf("gitea is not initialized")
	}
	setting.LoadSettings() // cannot access session settings otherwise

	verbose := ctx.Bool("verbose")
	if verbose && ctx.Bool("quiet") {
		return fmt.Errorf("--quiet and --verbose cannot both be set")
	}

	stdCtx, cancel := installSignals()
	defer cancel()

	err := db.InitEngine(stdCtx)
	if err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	if file == nil {
		file, err = os.Create(fileName)
		if err != nil {
			fatal("Unable to open %s: %v", fileName, err)
		}
	}
	defer file.Close()

	absFileName, err := filepath.Abs(fileName)
	if err != nil {
		return err
	}

	var iface any
	if fileName == "-" {
		iface, err = archiver.ByExtension(fmt.Sprintf(".%s", outType))
	} else {
		iface, err = archiver.ByExtension(fileName)
	}
	if err != nil {
		fatal("Unable to get archiver for extension: %v", err)
	}

	w, _ := iface.(archiver.Writer)
	if err := w.Create(file); err != nil {
		fatal("Creating archiver.Writer failed: %v", err)
	}
	defer w.Close()

	if ctx.IsSet("skip-repository") && ctx.Bool("skip-repository") {
		log.Info("Skip dumping local repositories")
	} else {
		log.Info("Dumping local repositories... %s", setting.RepoRootPath)
		if err := addRecursiveExclude(w, "repos", setting.RepoRootPath, []string{absFileName}, verbose); err != nil {
			fatal("Failed to include repositories: %v", err)
		}

		if ctx.IsSet("skip-lfs-data") && ctx.Bool("skip-lfs-data") {
			log.Info("Skip dumping LFS data")
		} else if !setting.LFS.StartServer {
			log.Info("LFS isn't enabled. Skip dumping LFS data")
		} else if err := storage.LFS.IterateObjects("", func(objPath string, object storage.Object) error {
			info, err := object.Stat()
			if err != nil {
				return err
			}

			return addReader(w, object, info, path.Join("data", "lfs", objPath), verbose)
		}); err != nil {
			fatal("Failed to dump LFS objects: %v", err)
		}
	}

	tmpDir := ctx.String("tempdir")
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		fatal("Path does not exist: %s", tmpDir)
	}

	dbDump, err := os.CreateTemp(tmpDir, "gitea-db.sql")
	if err != nil {
		fatal("Failed to create tmp file: %v", err)
	}
	defer func() {
		_ = dbDump.Close()
		if err := util.Remove(dbDump.Name()); err != nil {
			log.Warn("Unable to remove temporary file: %s: Error: %v", dbDump.Name(), err)
		}
	}()

	targetDBType := ctx.String("database")
	if len(targetDBType) > 0 && targetDBType != setting.Database.Type.String() {
		log.Info("Dumping database %s => %s...", setting.Database.Type, targetDBType)
	} else {
		log.Info("Dumping database...")
	}

	if err := db.DumpDatabase(dbDump.Name(), targetDBType); err != nil {
		fatal("Failed to dump database: %v", err)
	}

	if err := addFile(w, "gitea-db.sql", dbDump.Name(), verbose); err != nil {
		fatal("Failed to include gitea-db.sql: %v", err)
	}

	if len(setting.CustomConf) > 0 {
		log.Info("Adding custom configuration file from %s", setting.CustomConf)
		if err := addFile(w, "app.ini", setting.CustomConf, verbose); err != nil {
			fatal("Failed to include specified app.ini: %v", err)
		}
	}

	if ctx.IsSet("skip-custom-dir") && ctx.Bool("skip-custom-dir") {
		log.Info("Skipping custom directory")
	} else {
		customDir, err := os.Stat(setting.CustomPath)
		if err == nil && customDir.IsDir() {
			if is, _ := isSubdir(setting.AppDataPath, setting.CustomPath); !is {
				if err := addRecursiveExclude(w, "custom", setting.CustomPath, []string{absFileName}, verbose); err != nil {
					fatal("Failed to include custom: %v", err)
				}
			} else {
				log.Info("Custom dir %s is inside data dir %s, skipped", setting.CustomPath, setting.AppDataPath)
			}
		} else {
			log.Info("Custom dir %s doesn't exist, skipped", setting.CustomPath)
		}
	}

	isExist, err := util.IsExist(setting.AppDataPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", setting.AppDataPath, err)
	}
	if isExist {
		log.Info("Packing data directory...%s", setting.AppDataPath)

		var excludes []string
		if setting.SessionConfig.OriginalProvider == "file" {
			var opts session.Options
			if err = json.Unmarshal([]byte(setting.SessionConfig.ProviderConfig), &opts); err != nil {
				return err
			}
			excludes = append(excludes, opts.ProviderConfig)
		}

		if ctx.IsSet("skip-index") && ctx.Bool("skip-index") {
			excludes = append(excludes, setting.Indexer.RepoPath)
			excludes = append(excludes, setting.Indexer.IssuePath)
		}

		excludes = append(excludes, setting.RepoRootPath)
		excludes = append(excludes, setting.LFS.Storage.Path)
		excludes = append(excludes, setting.Attachment.Storage.Path)
		excludes = append(excludes, setting.Packages.Storage.Path)
		excludes = append(excludes, setting.Log.RootPath)
		excludes = append(excludes, absFileName)
		if err := addRecursiveExclude(w, "data", setting.AppDataPath, excludes, verbose); err != nil {
			fatal("Failed to include data directory: %v", err)
		}
	}

	if ctx.IsSet("skip-attachment-data") && ctx.Bool("skip-attachment-data") {
		log.Info("Skip dumping attachment data")
	} else if err := storage.Attachments.IterateObjects("", func(objPath string, object storage.Object) error {
		info, err := object.Stat()
		if err != nil {
			return err
		}

		return addReader(w, object, info, path.Join("data", "attachments", objPath), verbose)
	}); err != nil {
		fatal("Failed to dump attachments: %v", err)
	}

	if ctx.IsSet("skip-package-data") && ctx.Bool("skip-package-data") {
		log.Info("Skip dumping package data")
	} else if !setting.Packages.Enabled {
		log.Info("Packages isn't enabled. Skip dumping package data")
	} else if err := storage.Packages.IterateObjects("", func(objPath string, object storage.Object) error {
		info, err := object.Stat()
		if err != nil {
			return err
		}

		return addReader(w, object, info, path.Join("data", "packages", objPath), verbose)
	}); err != nil {
		fatal("Failed to dump packages: %v", err)
	}

	// Doesn't check if LogRootPath exists before processing --skip-log intentionally,
	// ensuring that it's clear the dump is skipped whether the directory's initialized
	// yet or not.
	if ctx.IsSet("skip-log") && ctx.Bool("skip-log") {
		log.Info("Skip dumping log files")
	} else {
		isExist, err := util.IsExist(setting.Log.RootPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", setting.Log.RootPath, err)
		}
		if isExist {
			if err := addRecursiveExclude(w, "log", setting.Log.RootPath, []string{absFileName}, verbose); err != nil {
				fatal("Failed to include log: %v", err)
			}
		}
	}

	if fileName != "-" {
		if err = w.Close(); err != nil {
			_ = util.Remove(fileName)
			fatal("Failed to save %s: %v", fileName, err)
		}

		if err := os.Chmod(fileName, 0o600); err != nil {
			log.Info("Can't change file access permissions mask to 0600: %v", err)
		}
	}

	if fileName != "-" {
		log.Info("Finish dumping in file %s", fileName)
	} else {
		log.Info("Finish dumping to stdout")
	}

	return nil
}

// addRecursiveExclude zips absPath to specified insidePath inside writer excluding excludeAbsPath
func addRecursiveExclude(w archiver.Writer, insidePath, absPath string, excludeAbsPath []string, verbose bool) error {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentAbsPath := path.Join(absPath, file.Name())
		currentInsidePath := path.Join(insidePath, file.Name())
		if file.IsDir() {
			if !util.SliceContainsString(excludeAbsPath, currentAbsPath) {
				if err := addFile(w, currentInsidePath, currentAbsPath, false); err != nil {
					return err
				}
				if err = addRecursiveExclude(w, currentInsidePath, currentAbsPath, excludeAbsPath, verbose); err != nil {
					return err
				}
			}
		} else {
			// only copy regular files and symlink regular files, skip non-regular files like socket/pipe/...
			shouldAdd := file.Mode().IsRegular()
			if !shouldAdd && file.Mode()&os.ModeSymlink == os.ModeSymlink {
				target, err := filepath.EvalSymlinks(currentAbsPath)
				if err != nil {
					return err
				}
				targetStat, err := os.Stat(target)
				if err != nil {
					return err
				}
				shouldAdd = targetStat.Mode().IsRegular()
			}
			if shouldAdd {
				if err = addFile(w, currentInsidePath, currentAbsPath, verbose); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
