package gen

import (
	"fmt"

	"github.com/atburke/krpc-go/api"
	"github.com/atburke/krpc-go/lib/utils"
	"github.com/dave/jennifer/jen"
	"github.com/mitchellh/go-wordwrap"
	"github.com/ztrue/tracerr"
)

const DocsLineLength = 77 // line length of 80 minus "// "

func GenerateService(f *jen.File, service *api.Service) error {
	return nil
}

func GenerateProcedure(f *jen.File, procedure *api.Procedure) error {
	return nil
}

func GenerateClass(f *jen.File, class *api.Class) error {
	return nil
}

func GenerateEnum(f *jen.File, enum *api.Enumeration) error {
	return nil
}

func GenerateException(f *jen.File, exception *api.Exception) error {
	// Names are given in the format XYZException. We want the more go-like
	// ErrXYZ.
	exceptionName := "Err" + exception.Name[:len(exception.Name)-len("exception")]
	docs, err := utils.ParseXMLDocumentation(exception.Documentation, exceptionName+" means ")
	if err != nil {
		return tracerr.Wrap(err)
	}

	// Define the error type.
	f.Comment(wordwrap.WrapString(docs, DocsLineLength))
	f.Type().Id(exceptionName).Struct(
		jen.Id("msg").String(),
	)

	// Define the constructor.
	constructorName := "New" + exceptionName
	f.Comment(fmt.Sprintf("%v creates a new %v.", constructorName, exceptionName))
	f.Func().Id(constructorName).Params(
		jen.Id("msg").String(),
	).Op("*").Id(exceptionName).Block(
		jen.Return(jen.Op("&").Id(exceptionName).Values(jen.Dict{
			jen.Id("msg"): jen.Id("msg"),
		})),
	)

	// Define the Error() function.
	f.Comment("Error returns a human-readable error.")
	f.Func().Params(
		jen.Id("err").Id(exceptionName),
	).Id("Error").Params().String().Block(
		jen.Return(jen.Id("err").Dot("msg")),
	)

	return nil
}
