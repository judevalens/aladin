package docsurface

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

// shardSDKSpec is the bare specifier for the nonvisual authored-shard runtime.
// It is vendored like the react family, but its source is embedded in this
// binary (it's OUR code) rather than fetched from esm.sh.
const shardSDKSpec = "@aladin/shard"

//go:embed shard-sdk.tsx
var shardSDKSource string

//go:embed resource-client.generated.js
var resourceClientSource string

// shardSDKSourceHash keys the shard SDK in the vendor cache, so a rebuilt binary with
// changed shard SDK source re-vendors (the served file stays content-addressed).
func shardSDKSourceHash() string {
	sum := sha256.Sum256([]byte(shardSDKSource + "\x00" + resourceClientSource))
	return hex.EncodeToString(sum[:])
}

// buildShardSDK bundles the embedded shard SDK into a single ESM file with the react family
// externalized — so the shard SDK, all shards, and React share ONE instance via the
// shard's import map.
func (b *builder) buildShardSDK() ([]byte, error) {
	res := esbuild.Build(esbuild.BuildOptions{
		Stdin: &esbuild.StdinOptions{
			Contents:   shardSDKSource,
			Loader:     esbuild.LoaderTSX,
			Sourcefile: "shard-sdk.tsx",
		},
		Bundle:           true,
		Write:            false,
		Format:           esbuild.FormatESModule,
		Platform:         esbuild.PlatformBrowser,
		Target:           esbuild.ES2020,
		JSX:              esbuild.JSXAutomatic,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Plugins:          []esbuild.Plugin{externalReactPlugin(), embeddedResourceClientPlugin()},
		LogLevel:         esbuild.LogLevelSilent,
	})
	if len(res.Errors) > 0 {
		return nil, fmt.Errorf("shard SDK build: %s", formatMessages(res.Errors))
	}
	if len(res.OutputFiles) == 0 {
		return nil, fmt.Errorf("shard SDK build produced no output")
	}
	return res.OutputFiles[0].Contents, nil
}

func embeddedResourceClientPlugin() esbuild.Plugin {
	return esbuild.Plugin{Name: "embedded-resource-client", Setup: func(build esbuild.PluginBuild) {
		build.OnResolve(esbuild.OnResolveOptions{Filter: `^\./resource-client\.generated\.js$`}, func(a esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
			return esbuild.OnResolveResult{Path: a.Path, Namespace: "resource-client"}, nil
		})
		build.OnLoad(esbuild.OnLoadOptions{Filter: ".*", Namespace: "resource-client"}, func(a esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
			return esbuild.OnLoadResult{Contents: &resourceClientSource, Loader: esbuild.LoaderJS}, nil
		})
	}}
}

// externalReactPlugin marks the react family external (react, react-dom,
// react/jsx-runtime, …) so the shard SDK bundle imports them at runtime via the import
// map instead of inlining a second copy.
func externalReactPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "external-react",
		Setup: func(build esbuild.PluginBuild) {
			build.OnResolve(esbuild.OnResolveOptions{Filter: `^react($|/|-dom)`},
				func(a esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					return esbuild.OnResolveResult{Path: a.Path, External: true}, nil
				})
		},
	}
}
