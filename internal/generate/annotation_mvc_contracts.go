package generate

import (
	"bytes"
	"go/ast"
	"strconv"
	"strings"
)

type mvcRouteConditions struct {
	Consumes []string
	Produces []string
	Params   []string
	Headers  []string
}

func mvcRouteConditionsFromAnnotation(annotation Annotation) mvcRouteConditions {
	return mvcRouteConditions{
		Consumes: mvcRouteConditionValues(annotation, "consumes"),
		Produces: mvcRouteConditionValues(annotation, "produces"),
		Params:   mvcRouteConditionValues(annotation, "params"),
		Headers:  mvcRouteConditionValues(annotation, "headers"),
	}
}

func mvcTypeRouteConditions(annotations []Annotation) mvcRouteConditions {
	for _, annotation := range annotations {
		if annotation.Name == "request-mapping" {
			return mvcRouteConditionsFromAnnotation(annotation)
		}
	}
	return mvcRouteConditions{}
}

func mvcRouteConditionValues(annotation Annotation, key string) []string {
	value := strings.TrimSpace(argString(annotation, key, ""))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func writeMVCRouteOptions(builder *bytes.Buffer, conditions mvcRouteConditions) {
	writeMVCRouteOption(builder, "Consumes", conditions.Consumes)
	writeMVCRouteOption(builder, "Produces", conditions.Produces)
	writeMVCRouteOption(builder, "Params", conditions.Params)
	writeMVCRouteOption(builder, "Headers", conditions.Headers)
}

func writeMVCControllerOptions(builder *bytes.Buffer, conditions mvcRouteConditions) {
	writeMVCControllerOption(builder, "Consumes", conditions.Consumes)
	writeMVCControllerOption(builder, "Produces", conditions.Produces)
	writeMVCControllerOption(builder, "Params", conditions.Params)
	writeMVCControllerOption(builder, "Headers", conditions.Headers)
}

func writeMVCRouteOption(builder *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString(", mvc.With")
	builder.WriteString(name)
	builder.WriteByte('(')
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(value))
	}
	builder.WriteByte(')')
}

func writeMVCControllerOption(builder *bytes.Buffer, name string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString(".With")
	builder.WriteString(name)
	builder.WriteByte('(')
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Quote(value))
	}
	builder.WriteByte(')')
}

func isArkartaMultipartPartExpr(file *ast.File, expr ast.Expr) bool {
	return isImportedSelectorExpr(file, expr, arkartaMultipartImportPath, "Part")
}
