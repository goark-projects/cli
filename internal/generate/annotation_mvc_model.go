package generate

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"goark.dev/cli/internal/generate/mvcrouting"
)

func mvcMultipartBodySelectorSet(annotations []Annotation) map[string]struct{} {
	selectors := mvcMultipartBodySelectors(annotations)
	out := make(map[string]struct{}, len(selectors))
	for _, selector := range selectors {
		out[selector] = struct{}{}
	}
	return out
}

func mvcMultipartBodySelectors(annotations []Annotation) []string {
	selectors := make([]string, 0, 1)
	for _, annotation := range annotations {
		if !isMVCMultipartBodyAnnotation(annotation.Name) {
			continue
		}
		if selector := mvcMultipartBodySelector(annotation); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	return selectors
}

func mvcMultipartBodySelector(annotation Annotation) string {
	selector := normalizeSelector(annotation.Selector)
	if selector != "" {
		return selector
	}
	for _, key := range []string{"param", "name", "value"} {
		if value := strings.TrimSpace(argString(annotation, key, "")); value != "" {
			return value
		}
	}
	return ""
}

func buildMVCController(
	fset *token.FileSet,
	typeSpec *ast.TypeSpec,
	annotations []Annotation,
) (*mvcController, error) {
	component, err := buildMVCComponent(fset, typeSpec, annotations, mvcControllerKind(annotations))
	if err != nil {
		return nil, err
	}
	crossOrigin, err := mvcCrossOriginFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	methods, err := mvcTypeRequestMethods(annotations)
	if err != nil {
		return nil, err
	}
	return &mvcController{
		Component:   component,
		BasePaths:   mvcTypeBasePaths(annotations),
		Methods:     methods,
		Kind:        mvcControllerKind(annotations),
		Conditions:  mvcTypeRouteConditions(annotations),
		CrossOrigin: crossOrigin,
	}, nil
}

func buildMVCComponent(
	fset *token.FileSet,
	typeSpec *ast.TypeSpec,
	annotations []Annotation,
	kind string,
) (annotationComponent, error) {
	typeName := typeSpec.Name.Name
	component := annotationComponent{
		TypeName: typeName,
		Name:     annotationName(annotations, kind, lowerCamel(typeName)),
	}
	structType, _ := typeSpec.Type.(*ast.StructType)
	if structType == nil {
		return component, nil
	}
	for _, field := range structType.Fields.List {
		fieldAnnotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return annotationComponent{}, err
		}
		if len(field.Names) == 0 {
			continue
		}
		for _, name := range field.Names {
			injection := buildInjection(fieldAnnotations, name.Name)
			if injection.Kind == "" {
				continue
			}
			component.Fields = append(component.Fields, annotationField{
				Name:      name.Name,
				Type:      exprString(fset, field.Type),
				Injection: injection,
			})
		}
	}
	return component, nil
}

func buildMVCRoute(
	fset *token.FileSet,
	file *ast.File,
	fn *ast.FuncDecl,
	annotations []Annotation,
) (mvcRoute, error) {
	mapping, err := mvcRouteFromAnnotations(annotations)
	if err != nil {
		return mvcRoute{}, err
	}
	handler, err := analyzeMVCHandler(fset, file, fn, annotations)
	if err != nil {
		return mvcRoute{}, err
	}
	return mvcRoute{
		HTTPMethod:       mapping.methods[0],
		HTTPMethods:      mapping.methods,
		HTTPMethodsSet:   mapping.methodsSet,
		Path:             mapping.paths[0],
		Paths:            mapping.paths,
		Status:           mapping.status,
		StatusExplicit:   mapping.explicitStatus,
		ResponseBody:     mapping.responseBody,
		ValidationGroups: mapping.validationGroups,
		Conditions:       mapping.conditions,
		CrossOrigin:      mapping.crossOrigin,
		Handler:          handler,
	}, nil
}

func buildMVCModelAttributeMethod(
	fset *token.FileSet,
	file *ast.File,
	fn *ast.FuncDecl,
	annotations []Annotation,
) (mvcModelAttributeMethod, error) {
	params, err := mvcModelAttributeMethodParams(file, fn)
	if err != nil {
		return mvcModelAttributeMethod{}, err
	}
	returnKind, err := mvcMethodReturnKind(file, fn)
	if err != nil {
		return mvcModelAttributeMethod{}, err
	}
	if !mvcModelAttributeMethodSupportsReturn(returnKind) {
		return mvcModelAttributeMethod{}, fmt.Errorf(
			"mvc model attribute method %s must return T or T,error",
			fn.Name.Name,
		)
	}
	return mvcModelAttributeMethod{
		Name:       mvcModelAttributeMethodNameAnnotation(annotations),
		Params:     params,
		ReturnKind: returnKind,
		ReturnType: mvcPrimaryReturnType(fset, fn),
	}, nil
}

type mvcRouteMappingSpec struct {
	methods          []string
	methodsSet       bool
	paths            []string
	status           int
	explicitStatus   bool
	responseBody     bool
	validationGroups []string
	conditions       mvcRouteConditions
	crossOrigin      *mvcCrossOrigin
}

func mvcRouteFromAnnotations(annotations []Annotation) (mvcRouteMappingSpec, error) {
	var out mvcRouteMappingSpec
	responseStatus := 0
	hasResponseStatus := false
	hasResponseBody := false
	hasValidated := false
	hasMapping := false
	for _, annotation := range annotations {
		if isMVCValidatedAnnotation(annotation.Name) {
			if hasValidated {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple validated annotations")
			}
			out.validationGroups = mvcValidationGroups(annotation)
			hasValidated = true
			continue
		}
		if isMVCResponseBodyAnnotation(annotation.Name) {
			if hasResponseBody {
				return mvcRouteMappingSpec{}, fmt.Errorf(
					"mvc route method has multiple response-body annotations",
				)
			}
			out.responseBody = true
			hasResponseBody = true
			continue
		}
		if isMVCResponseStatusAnnotation(annotation.Name) {
			if hasResponseStatus {
				return mvcRouteMappingSpec{}, fmt.Errorf(
					"mvc route method has multiple response-status annotations",
				)
			}
			status, err := mvcResponseStatus(annotation)
			if err != nil {
				return mvcRouteMappingSpec{}, err
			}
			responseStatus = status
			hasResponseStatus = true
			continue
		}
		if isMVCRouteMappingAnnotation(annotation.Name) {
			if hasMapping {
				return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method has multiple mapping annotations")
			}
			mapping, err := mvcRouteMapping(annotation)
			if err != nil {
				return mvcRouteMappingSpec{}, err
			}
			out = mapping
			hasMapping = true
		}
	}
	crossOrigin, err := mvcCrossOriginFromAnnotations(annotations)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	out.crossOrigin = crossOrigin
	if !hasMapping {
		return mvcRouteMappingSpec{}, fmt.Errorf("mvc route method requires mapping annotation")
	}
	if hasResponseStatus {
		if out.explicitStatus {
			return mvcRouteMappingSpec{}, fmt.Errorf(
				"mvc route method must not declare both mapping status and response-status",
			)
		}
		out.status = responseStatus
		out.explicitStatus = true
	}
	return out, nil
}

func mvcRouteMapping(annotation Annotation) (mvcRouteMappingSpec, error) {
	paths, err := requireMVCPathTexts(annotation)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	methods, methodsSet, err := mvcHTTPMethods(annotation)
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	explicitStatus := mvcMappingHasExplicitStatus(annotation)
	status, err := mvcStatus(annotation, mvcrouting.DefaultStatus(methods))
	if err != nil {
		return mvcRouteMappingSpec{}, err
	}
	return mvcRouteMappingSpec{
		methods:        methods,
		methodsSet:     methodsSet,
		paths:          mvcrouting.NormalizePaths(paths),
		status:         status,
		explicitStatus: explicitStatus,
		conditions:     mvcRouteConditionsFromAnnotation(annotation),
	}, nil
}

func mvcParameterMapFunction(kind mvcHandlerParamKind, typ string) (string, bool) {
	switch strings.TrimSpace(typ) {
	case "map[string]string":
		switch kind {
		case mvcParamRequestParam:
			return "RequestParamMap", true
		case mvcParamRequestHeader:
			return "RequestHeaderMap", true
		default:
			return "", false
		}
	case "map[string][]string":
		switch kind {
		case mvcParamRequestParam:
			return "RequestParamValuesMap", true
		case mvcParamRequestHeader:
			return "RequestHeaderValuesMap", true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func validateMVCParameterMapBinding(
	methodName, paramName, typ string,
	kind mvcHandlerParamKind,
	binding mvcParamBinding,
) error {
	if _, ok := mvcParameterMapFunction(kind, typ); !ok {
		return nil
	}
	if binding.SourceExplicit || binding.HasDefault || !binding.Required {
		return mvcHandlerError(
			"map parameter %s must not declare name, value, defaultValue, or required=false",
			methodName,
			paramName,
		)
	}
	return nil
}

type mvcParameterBindingItem struct {
	Kind    mvcHandlerParamKind
	Binding mvcParamBinding
}

func mvcParameterBindingSet(annotations []Annotation) (map[string]mvcParameterBindingItem, error) {
	out := make(map[string]mvcParameterBindingItem)
	for _, annotation := range annotations {
		kind, ok := mvcParameterKind(annotation.Name)
		if !ok {
			continue
		}
		selector := mvcBindingSelector(annotation)
		if selector == "" {
			continue
		}
		if _, exists := out[selector]; exists {
			return nil, fmt.Errorf("mvc parameter %q has multiple binding annotations", selector)
		}
		out[selector] = mvcParameterBindingItem{
			Kind: kind,
			Binding: mvcParamBinding{
				SourceName:     mvcParameterSourceName(annotation, selector),
				SourceExplicit: mvcParameterHasSourceName(annotation),
				Required:       mvcParameterRequired(annotation),
				HasDefault:     mvcParameterHasDefault(annotation),
				DefaultValue:   mvcParameterDefaultValue(annotation),
			},
		}
	}
	return out, nil
}
