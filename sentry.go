package main

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

func sentryInit(release, DSN string) {
	err := sentry.Init(sentry.ClientOptions{
		// Either set your DSN here or set the SENTRY_DSN environment variable.
		Dsn:              DSN,
		Release:          release,
		AttachStacktrace: true,
	})
	if err != nil {
		capture(err)
	}
}

func filterFrames(frames []sentry.Frame) []sentry.Frame {
	if len(frames) == 0 {
		return nil
	}
	filteredFrames := make([]sentry.Frame, 0, len(frames))
	for _, frame := range frames {
		if frame.Module == "runtime" || frame.Module == "testing" {
			continue
		}
		if frame.Module == "main" && strings.HasPrefix(frame.Function, "capture") {
			continue
		}
		filteredFrames = append(filteredFrames, frame)
	}
	return filteredFrames
}

var sentryHandler = sentryhttp.New(sentryhttp.Options{
	Repanic:         false,
	WaitForDelivery: true,
})

func captureGetRequestForContext(r *http.Request) sentry.Context {
	return sentry.Context{
		"Method":     r.Method,
		"URL":        r.URL.String(),
		"RemoteAddr": r.RemoteAddr,
	}
}

func capture(err error) bool {
	if err == nil {
		return false
	}
	client, scope, event := captureGetEvent(err)
	go client.CaptureEvent(event, &sentry.EventHint{OriginalException: err}, scope)
	go fmt.Println(err.Error())
	return true
}

func captureGetEvent(err error) (*sentry.Client, *sentry.Scope, *sentry.Event) {
	extractFrames := func(pcs []uintptr) []sentry.Frame {
		var frames []sentry.Frame
		callersFrames := runtime.CallersFrames(pcs)
		for {
			callerFrame, more := callersFrames.Next()

			frames = append([]sentry.Frame{
				sentry.NewFrame(callerFrame),
			}, frames...)

			if !more {
				break
			}
		}
		return frames
	}
	GetStacktrace := func() *sentry.Stacktrace {
		pcs := make([]uintptr, 100)
		n := runtime.Callers(1, pcs)
		if n == 0 {
			return nil
		}
		frames := extractFrames(pcs[:n])
		frames = filterFrames(frames)
		stacktrace := sentry.Stacktrace{
			Frames: frames,
		}
		return &stacktrace
	}
	if err == nil {
		return nil, nil, nil
	}
	event := sentry.NewEvent()
	event.Exception = append(event.Exception, sentry.Exception{
		Value:      err.Error(),
		Type:       reflect.TypeOf(err).String(),
		Stacktrace: GetStacktrace(),
	})
	event.Level = sentry.LevelError
	hub := sentry.CurrentHub()
	return hub.Client(), hub.Scope(), event
}

func captureFunc(f func() error) bool {
	err := f()
	return capture(err)
}

func captureDouble(k interface{}, err error) bool {
	go func() {
		switch v := k.(type) {
		case *http.Response:
			closeBody(v)
		default:
		}
	}()
	return capture(err)
}
func closeBody(resp *http.Response) {
	if resp == nil {
		return
	}
	if resp.Body == nil {
		return
	}
	capture(resp.Body.Close())
}
func capture2(err error, errorContext *http.Request) bool {
	if err == nil {
		return false
	}
	if errorContext == nil {
		go capture(err)
		return true
	}
	client, scope, event := captureGetEvent(err)
	localScope := scope.Clone()
	localScope.SetContext("Request", captureGetRequestForContext(errorContext))
	go client.CaptureEvent(event, &sentry.EventHint{OriginalException: err}, localScope)
	return true
}
