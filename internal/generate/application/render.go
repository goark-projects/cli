package application

import (
	"strconv"
	"strings"

	"goark.dev/cli/internal/generate"
)

func addImports(ctx *generate.AnnotationGenerationContext, web bool) {
	ctx.AddImport("", "context")
	ctx.AddImport("", "fmt")
	ctx.AddImport("", "os")
	ctx.AddImport("", "log/slog")
	ctx.AddImport("", "time")
	ctx.AddImport("", "goark.dev/boot")
	ctx.AddImport("", "goark.dev/boot/configdata")
	ctx.AddImport("gbclog", "goark.dev/gbc-log")
	if !web {
		return
	}
	ctx.AddImport("", "os/signal")
	ctx.AddImport("", "syscall")
	ctx.AddImport("gbcarkhos", "goark.dev/gbc-arkhos")
	ctx.AddImport("gbcweb", "goark.dev/gbc-web")
	ctx.AddImport("", "goark.dev/goark")
}

func writeRun(
	ctx *generate.AnnotationGenerationContext,
	web bool,
	configurationTypes []string,
) {
	var source strings.Builder
	source.WriteString("// Run 启动当前 Goark Boot 应用。\n")
	source.WriteString("func Run(args []string) (exitCode int) {\n")
	writeContext(&source, web)
	source.WriteString("application, err := boot.Run(ctx,\n")
	source.WriteString("boot.WithConfigDataOptions(configdata.WithArgs(args...)),\n")
	source.WriteString("boot.WithConfiguration(")
	for _, name := range configurationTypes {
		source.WriteString(name)
		source.WriteString("{},")
	}
	source.WriteString("),\nboot.WithAutoConfiguration(")
	source.WriteString("gbclog.AutoConfigure()")
	if web {
		source.WriteString(",gbcweb.AutoConfigure()")
	}
	source.WriteString("),\n)\n")
	source.WriteString("if err != nil {\n")
	source.WriteString("fmt.Fprintln(os.Stderr,")
	if web {
		source.WriteString(strconv.Quote("启动 Goark Boot Web 应用失败"))
	} else {
		source.WriteString(strconv.Quote("启动 Goark Boot 应用失败"))
	}
	source.WriteString(",\"error\",err)\nreturn 1\n}\n")
	writeClose(&source, web)
	if web {
		writeWebWait(&source)
	} else {
		source.WriteString("slog.Info(\"应用已启动\")\nreturn 0\n")
	}
	source.WriteString("}\n")
	ctx.WriteString(source.String())
}

func writeContext(source *strings.Builder, web bool) {
	if web {
		source.WriteString(
			"ctx, stop := signal.NotifyContext(context.Background(), " +
				"os.Interrupt, syscall.SIGTERM)\ndefer stop()\n",
		)
		return
	}
	source.WriteString("ctx := context.Background()\n")
}

func writeClose(source *strings.Builder, web bool) {
	source.WriteString("defer func() {\n")
	source.WriteString(
		"shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)\n" +
			"defer cancel()\nif closeErr := application.Close(shutdownCtx); closeErr != nil {\n",
	)
	message := "关闭 Goark Boot 应用失败"
	if web {
		message = "关闭 Goark Boot Web 应用失败"
	}
	source.WriteString("slog.Error(" + strconv.Quote(message) + ",\"error\",closeErr)\n")
	source.WriteString("exitCode = 1\n}\n}()\n")
}

func writeWebWait(source *strings.Builder) {
	source.WriteString("appContext, ok := application.Context()\n")
	source.WriteString("if !ok {\nslog.Error(\"应用上下文尚未初始化\")\nreturn 1\n}\n")
	source.WriteString(
		"server, err := goark.Get[*gbcarkhos.EmbeddedServer](ctx, appContext, " +
			"gbcarkhos.BeanNameServer)\n",
	)
	source.WriteString(
		"if err != nil {\nslog.Error(\"获取嵌入式服务失败\",\"error\",err)\nreturn 1\n}\n",
	)
	source.WriteString("slog.Info(\"服务已启动\",\"url\",server.URL())\n<-ctx.Done()\nreturn 0\n")
}
