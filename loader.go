package lamvms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

// DefaultMicrovmFiles is the list of default file names to search for.
var DefaultMicrovmFiles = []string{
	"microvm.jsonnet",
	"microvm.json",
}

// Loader loads and evaluates microvm definition files.
type Loader struct {
	extStr      map[string]string
	extCode     map[string]string
	nativeFuncs []*jsonnet.NativeFunction
	funcMap     template.FuncMap
}

// NewLoader creates a Loader with Jsonnet native functions and template functions.
func NewLoader(ctx context.Context, awsCfg aws.Config, extStr, extCode map[string]string) *Loader {
	callerID := newCallerIdentity(awsCfg, ctx)

	nativeFuncs := []*jsonnet.NativeFunction{
		nativeFuncEnv(),
		nativeFuncMustEnv(),
	}
	nativeFuncs = append(nativeFuncs, callerID.jsonnetNativeFuncs()...)

	funcMap := template.FuncMap{
		"env":      templateFuncEnv,
		"must_env": templateFuncMustEnv,
	}
	maps.Copy(funcMap, callerID.templateFuncMap())

	return &Loader{
		extStr:      extStr,
		extCode:     extCode,
		nativeFuncs: nativeFuncs,
		funcMap:     funcMap,
	}
}

// Load loads a MicrovmImage from the given path and returns the resolved path.
// If path is empty, it searches for default files.
func (l *Loader) Load(path string) (*MicrovmImage, string, error) {
	var err error
	if path == "" {
		path, err = findMicrovmFile()
		if err != nil {
			return nil, "", err
		}
	}

	expanded, err := l.loadAndExpand(path)
	if err != nil {
		return nil, "", err
	}

	var img MicrovmImage
	if err := json.Unmarshal(expanded, &img); err != nil {
		return nil, "", fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &img, path, nil
}

// LoadRunConfig loads a RunConfig from the given path.
func (l *Loader) LoadRunConfig(path string) (*RunConfig, error) {
	expanded, err := l.loadAndExpand(path)
	if err != nil {
		return nil, err
	}

	var rc RunConfig
	if err := json.Unmarshal(expanded, &rc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &rc, nil
}

func (l *Loader) loadAndExpand(path string) ([]byte, error) {
	slog.Info("loading definition", "path", path)

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	if filepath.Ext(path) == ".jsonnet" {
		slog.Debug("evaluating jsonnet")
		vm := l.jsonnetVM()
		evaluated, err := vm.EvaluateAnonymousSnippet(path, string(src))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet %s: %w", path, err)
		}
		return []byte(evaluated), nil
	}

	return l.expandTemplate(src)
}

func findMicrovmFile() (string, error) {
	for _, name := range DefaultMicrovmFiles {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no microvm definition file found (searched: %v)", DefaultMicrovmFiles)
}

func (l *Loader) jsonnetVM() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	for k, v := range l.extStr {
		vm.ExtVar(k, v)
	}
	for k, v := range l.extCode {
		vm.ExtCode(k, v)
	}
	for _, f := range l.nativeFuncs {
		vm.NativeFunction(f)
	}
	return vm
}

func nativeFuncEnv() *jsonnet.NativeFunction {
	return &jsonnet.NativeFunction{
		Name:   "env",
		Params: ast.Identifiers{"name", "default"},
		Func: func(args []any) (any, error) {
			name, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("env: name must be a string")
			}
			if v, ok := os.LookupEnv(name); ok {
				return v, nil
			}
			if args[1] == nil {
				return "", nil
			}
			def, ok := args[1].(string)
			if !ok {
				return nil, fmt.Errorf("env: default must be a string")
			}
			return def, nil
		},
	}
}

func nativeFuncMustEnv() *jsonnet.NativeFunction {
	return &jsonnet.NativeFunction{
		Name:   "must_env",
		Params: ast.Identifiers{"name"},
		Func: func(args []any) (any, error) {
			name, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("must_env: name must be a string")
			}
			v, ok := os.LookupEnv(name)
			if !ok {
				return nil, fmt.Errorf("must_env: environment variable %q is not set", name)
			}
			return v, nil
		},
	}
}

func (l *Loader) expandTemplate(src []byte) ([]byte, error) {
	tmpl, err := template.New("microvm").Funcs(l.funcMap).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func templateFuncEnv(name string, defaultValues ...string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	if len(defaultValues) > 0 {
		return defaultValues[0]
	}
	return ""
}

func templateFuncMustEnv(name string) (string, error) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("environment variable %q is not set", name)
	}
	return v, nil
}
