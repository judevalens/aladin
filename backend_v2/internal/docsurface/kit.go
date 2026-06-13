package docsurface

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

// kitSpec is the bare specifier agents import: `import { Region } from "@aladin/kit"`.
// It is vendored like the react family, but its source is embedded in this
// binary (it's OUR code) rather than fetched from esm.sh.
const kitSpec = "@aladin/kit"

//go:embed kit.tsx
var kitSource string

// kitSourceHash keys the kit in the vendor cache, so a rebuilt binary with
// changed kit source re-vendors (the served file stays content-addressed).
func kitSourceHash() string {
	sum := sha256.Sum256([]byte(kitSource))
	return hex.EncodeToString(sum[:])
}

// buildKit bundles the embedded kit into a single ESM file with the react family
// externalized — so the kit, all shards, and React share ONE instance via the
// shard's import map.
func (b *builder) buildKit() ([]byte, error) {
	res := esbuild.Build(esbuild.BuildOptions{
		Stdin: &esbuild.StdinOptions{
			Contents:   kitSource,
			Loader:     esbuild.LoaderTSX,
			Sourcefile: "kit.tsx",
		},
		Bundle:           true,
		Write:            false,
		Format:           esbuild.FormatESModule,
		Platform:         esbuild.PlatformBrowser,
		Target:           esbuild.ES2020,
		JSX:              esbuild.JSXAutomatic,
		MinifyWhitespace: true,
		MinifySyntax:     true,
		Plugins:          []esbuild.Plugin{externalReactPlugin()},
		LogLevel:         esbuild.LogLevelSilent,
	})
	if len(res.Errors) > 0 {
		return nil, fmt.Errorf("kit build: %s", formatMessages(res.Errors))
	}
	if len(res.OutputFiles) == 0 {
		return nil, fmt.Errorf("kit build produced no output")
	}
	return res.OutputFiles[0].Contents, nil
}

// externalReactPlugin marks the react family external (react, react-dom,
// react/jsx-runtime, …) so the kit bundle imports them at runtime via the import
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
