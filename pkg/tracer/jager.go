package tracer

import (
	"fmt"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	jaegercfg "github.com/uber/jaeger-client-go/config"
)

func NewTracer() (opentracing.Tracer, io.Closer, error) {
	// load config from environment variables
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not parse Jaeger env vars")
	}

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not initialize Jaeger tracer")
	}

	helloTo := "hi"

	span := tracer.StartSpan("say-hello")
	span.SetTag("hello-to", helloTo)

	helloStr := fmt.Sprintf("Hello, %s!", helloTo)
	span.LogFields(
		log.String("event", "string-format"),
		log.String("value", helloStr),
	)

	println(helloStr)
	span.LogKV("event", "println")

	span.Finish()

	return tracer, closer, nil
}
