// codegen generates Go types with custom (Un)MarshalJSON for AWS SDK structs
// that contain union-typed fields. Also generates test fixtures.
// Based on the approach from mashiike/acrun:
// https://github.com/mashiike/acrun/tree/main/cmd/codegen
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/tools/go/packages"
)

//go:embed template.go.tmpl
var templateContent string

const (
	sdkPkgPath   = "github.com/aws/aws-sdk-go-v2/service/lambdamicrovms"
	typesPkgPath = sdkPkgPath + "/types"
	outputFile   = "aws.gen.go"
	fixtureDir   = "testdata/gen"
)

// TargetStruct defines a struct to generate code for.
type TargetStruct struct {
	SDKName     string // e.g. "CreateMicrovmImageInput"
	GoName      string // e.g. "MicrovmImage"
	HasFixtures bool
}

// UpdateMapping defines a mapping from a generated type to an Update SDK input.
type UpdateMapping struct {
	CreateSDKName string // e.g. "CreateMicrovmImageInput"
	GoName        string // source type, e.g. "MicrovmImage"
	UpdateSDKName string // e.g. "UpdateMicrovmImageInput"
	IDField       string // e.g. "ImageIdentifier"
	SkipFields    map[string]bool
}

// CommonField represents a field shared between Create and Update inputs.
type CommonField struct {
	Name string
}

var updateMappings = []UpdateMapping{
	{
		CreateSDKName: "CreateMicrovmImageInput",
		GoName:        "MicrovmImage",
		UpdateSDKName: "UpdateMicrovmImageInput",
		IDField:       "ImageIdentifier",
		SkipFields:    map[string]bool{"Name": true, "ClientToken": true, "Tags": true},
	},
}

var targets = []TargetStruct{
	{SDKName: "CreateMicrovmImageInput", GoName: "MicrovmImage", HasFixtures: true},
	{SDKName: "RunMicrovmInput", GoName: "RunConfig", HasFixtures: false},
}

// StructInfo holds all info needed to generate code for one struct.
type StructInfo struct {
	SDKName     string
	GoName      string
	UnionFields []UnionFieldInfo
	HasFixtures bool
}

// UnionFieldInfo holds metadata about a union type field.
type UnionFieldInfo struct {
	FieldName     string
	InterfaceType string
	Members       []MemberInfo
}

// MemberInfo holds metadata about a concrete union member.
type MemberInfo struct {
	TypeName       string
	FieldName      string
	JSONKey        string
	ValueTypeShort string
	IsDirectValue  bool
	FixtureFile    string
}

func main() {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedName,
	}
	pkgs, err := packages.Load(cfg, sdkPkgPath, typesPkgPath)
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Fatalf("package errors encountered")
	}

	var basePkg, typesPkg *packages.Package
	for _, p := range pkgs {
		switch p.PkgPath {
		case sdkPkgPath:
			basePkg = p
		case typesPkgPath:
			typesPkg = p
		}
	}
	if basePkg == nil || typesPkg == nil {
		log.Fatalf("required packages not found")
	}

	var structs []StructInfo
	for _, target := range targets {
		si := scanStruct(basePkg, typesPkg, target)
		structs = append(structs, si)
	}

	allUnions := deduplicateUnions(structs)

	var updateInfos []UpdateMappingInfo
	for _, um := range updateMappings {
		info := scanUpdateMapping(basePkg, um)
		updateInfos = append(updateInfos, info)
	}

	if err := generateCode(structs, allUnions, updateInfos); err != nil {
		log.Fatalf("failed to generate code: %v", err)
	}
	if err := generateFixtures(structs); err != nil {
		log.Fatalf("failed to generate fixtures: %v", err)
	}
	log.Printf("generated %s for %d structs", outputFile, len(structs))
}

// UpdateMappingInfo holds the result of scanning an Update mapping.
type UpdateMappingInfo struct {
	GoName        string
	UpdateSDKName string
	IDField       string
	CommonFields  []CommonField
}

func scanUpdateMapping(basePkg *packages.Package, um UpdateMapping) UpdateMappingInfo {
	createObj := basePkg.Types.Scope().Lookup(um.CreateSDKName)
	if createObj == nil {
		log.Fatalf("%s not found", um.CreateSDKName)
	}
	createStruct, ok := createObj.Type().Underlying().(*types.Struct)
	if !ok {
		log.Fatalf("%s is not a struct", um.CreateSDKName)
	}

	updateObj := basePkg.Types.Scope().Lookup(um.UpdateSDKName)
	if updateObj == nil {
		log.Fatalf("%s not found", um.UpdateSDKName)
	}
	updateStruct, ok := updateObj.Type().Underlying().(*types.Struct)
	if !ok {
		log.Fatalf("%s is not a struct", um.UpdateSDKName)
	}

	updateFields := make(map[string]bool)
	for i := 0; i < updateStruct.NumFields(); i++ {
		f := updateStruct.Field(i)
		if f.Exported() {
			updateFields[f.Name()] = true
		}
	}

	info := UpdateMappingInfo{
		GoName:        um.GoName,
		UpdateSDKName: um.UpdateSDKName,
		IDField:       um.IDField,
	}
	coveredFields := map[string]bool{um.IDField: true}
	for k := range um.SkipFields {
		coveredFields[k] = true
	}
	for i := 0; i < createStruct.NumFields(); i++ {
		f := createStruct.Field(i)
		if !f.Exported() {
			continue
		}
		if um.SkipFields[f.Name()] {
			continue
		}
		if updateFields[f.Name()] {
			info.CommonFields = append(info.CommonFields, CommonField{Name: f.Name()})
			coveredFields[f.Name()] = true
		}
	}

	var uncovered []string
	for name := range updateFields {
		if !coveredFields[name] {
			uncovered = append(uncovered, name)
		}
	}
	if len(uncovered) > 0 {
		log.Fatalf("%s has fields not covered by %s or SkipFields: %v\nAdd them to SkipFields or the source struct.",
			um.UpdateSDKName, um.CreateSDKName, uncovered)
	}
	return info
}

func scanStruct(basePkg, typesPkg *packages.Package, target TargetStruct) StructInfo {
	obj := basePkg.Types.Scope().Lookup(target.SDKName)
	if obj == nil {
		log.Fatalf("%s not found in package", target.SDKName)
	}
	struc, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		log.Fatalf("%s is not a struct", target.SDKName)
	}

	si := StructInfo{
		SDKName:     target.SDKName,
		GoName:      target.GoName,
		HasFixtures: target.HasFixtures,
	}

	for i := 0; i < struc.NumFields(); i++ {
		field := struc.Field(i)
		fieldType := field.Type()
		iface, ok := fieldType.Underlying().(*types.Interface)
		if !ok {
			continue
		}

		info := UnionFieldInfo{
			FieldName:     field.Name(),
			InterfaceType: fieldType.String(),
		}

		for _, name := range typesPkg.Types.Scope().Names() {
			obj := typesPkg.Types.Scope().Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if tn.Name() == "UnknownUnionMember" {
				continue
			}
			pt := types.NewPointer(tn.Type())
			if !types.Implements(pt, iface) {
				continue
			}
			if types.Identical(tn.Type(), fieldType) {
				continue
			}
			if strct, ok := tn.Type().Underlying().(*types.Struct); ok {
				info.Members = append(info.Members, extractMember(info.FieldName, tn.Name(), strct))
			}
		}
		si.UnionFields = append(si.UnionFields, info)
	}
	return si
}

func deduplicateUnions(structs []StructInfo) []UnionFieldInfo {
	seen := map[string]bool{}
	var result []UnionFieldInfo
	for _, s := range structs {
		for _, uf := range s.UnionFields {
			if !seen[uf.FieldName] {
				seen[uf.FieldName] = true
				result = append(result, uf)
			}
		}
	}
	return result
}

func generateCode(structs []StructInfo, allUnions []UnionFieldInfo, updateInfos []UpdateMappingInfo) error {
	tmpl, err := template.New("codegen").Funcs(template.FuncMap{
		"toLowerCamel": toLowerCamel,
	}).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	data := map[string]any{
		"Structs":        structs,
		"AllUnions":      allUnions,
		"UpdateMappings": updateInfos,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format source: %w\n%s", err, buf.String())
	}

	return os.WriteFile(outputFile, formatted, 0644)
}

func generateFixtures(structs []StructInfo) error {
	if err := os.MkdirAll(fixtureDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", fixtureDir, err)
	}

	for _, s := range structs {
		if !s.HasFixtures {
			continue
		}
		for _, uf := range s.UnionFields {
			for _, m := range uf.Members {
				fixture := buildFixture(uf.FieldName, m)
				data, err := json.MarshalIndent(fixture, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal fixture for %s: %w", m.TypeName, err)
				}
				data = append(data, '\n')
				if err := os.WriteFile(m.FixtureFile, data, 0644); err != nil {
					return fmt.Errorf("write fixture %s: %w", m.FixtureFile, err)
				}
			}
		}
	}
	return nil
}

func buildFixture(fieldName string, m MemberInfo) map[string]any {
	fixture := map[string]any{
		"Name":         "gen-test-" + m.JSONKey,
		"BaseImageArn": "arn:aws:lambda:ap-northeast-1:aws:microvm-image:al2023-1",
		"BuildRoleArn": "arn:aws:iam::123456789012:role/TestBuildRole",
	}

	if fieldName == "CodeArtifact" {
		fixture["CodeArtifact"] = buildVariantValue(m)
	} else {
		fixture["CodeArtifact"] = map[string]any{"uri": "s3://test-bucket/artifact.zip"}
		fixture[fieldName] = map[string]any{m.JSONKey: buildMemberTestValue(m)}
	}
	return fixture
}

func buildVariantValue(m MemberInfo) map[string]any {
	return map[string]any{m.JSONKey: buildMemberTestValue(m)}
}

func buildMemberTestValue(m MemberInfo) any {
	if m.IsDirectValue {
		return "s3://gen-test-bucket/artifact.zip"
	}
	return map[string]any{"logGroup": "/aws/lambda/gen-test"}
}

func extractMember(parentField, typeName string, strct *types.Struct) MemberInfo {
	jsonKey := inferJSONKey(typeName)
	m := MemberInfo{
		TypeName:    typeName,
		FieldName:   toUpperCamel(jsonKey),
		JSONKey:     jsonKey,
		FixtureFile: fmt.Sprintf("%s/%s_%s.json", fixtureDir, toLowerCamel(parentField), jsonKey),
	}
	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)
		if f.Name() == "Value" {
			m.ValueTypeShort = shortType(f.Type().String())
			_, m.IsDirectValue = f.Type().Underlying().(*types.Basic)
			if _, ok := f.Type().Underlying().(*types.Slice); ok {
				m.IsDirectValue = true
			}
			break
		}
	}
	return m
}

func inferJSONKey(typeName string) string {
	parts := strings.Split(typeName, "Member")
	if len(parts) != 2 {
		return toLowerCamel(typeName)
	}
	return toLowerCamel(parts[1])
}

func shortType(full string) string {
	return strings.ReplaceAll(full, typesPkgPath+".", "types.")
}

func toLowerCamel(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func toUpperCamel(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
