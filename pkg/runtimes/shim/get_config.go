package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/casext"
)

func getConfig(args []string) error {
	ctx := context.Background()

	if len(args) != 3 {
		return fmt.Errorf("usage: get-config image.tar tag dest/")
	}

	archiveSrc := args[0]
	fromName := args[1]
	configDst := args[2]

	layout, err := openTar(archiveSrc)
	if err != nil {
		return fmt.Errorf("create layout: %w", err)
	}

	defer layout.Close()

	ext := casext.NewEngine(layout)

	mspec, err := loadManifest(ctx, ext, fromName)
	if err != nil {
		return err
	}

	config, err := ext.FromDescriptor(ctx, mspec.Config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if config.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return fmt.Errorf("bad config media type: %s", config.Descriptor.MediaType)
	}

	ispec := config.Data.(ispec.Image)

	configPath := filepath.Join(configDst, "config.json")

	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("create config.json: %w", err)
	}

	defer configFile.Close()

	err = json.NewEncoder(configFile).Encode(ispec.Config)
	if err != nil {
		return fmt.Errorf("encode image config: %w", err)
	}

	return nil
}

func loadManifest(ctx context.Context, ext casext.Engine, name string) (ispec.Manifest, error) {
	descPaths, err := ext.ResolveReference(context.Background(), name)
	if err != nil {
		return ispec.Manifest{}, fmt.Errorf("resolve ref: %w", err)
	}

	if len(descPaths) == 0 {
		return ispec.Manifest{}, fmt.Errorf("tag not found: %s", name)
	}

	if len(descPaths) != 1 {
		return ispec.Manifest{}, fmt.Errorf("ambiguous tag?: %s (%d paths returned)", name, len(descPaths))
	}

	manifest, err := ext.FromDescriptor(ctx, descPaths[0].Descriptor())
	if err != nil {
		return ispec.Manifest{}, fmt.Errorf("load manifest: %w", err)
	}

	if manifest.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return ispec.Manifest{}, fmt.Errorf("bad manifest media type: %s", manifest.Descriptor.MediaType)
	}

	return manifest.Data.(ispec.Manifest), nil
}
