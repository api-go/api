package api

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"apigo.cc/go/cast"
	"apigo.cc/go/log"
	"apigo.cc/go/safe"
)

const logTypeAPICall = "api_call"

type CallLog struct {
	log.BaseLog
	Action       string  `log:"pos:6,color:cyan"`
	Method       string  `log:"pos:7,color:gray"`
	URL          string  `log:"pos:8,color:gray"`
	StatusCode   int     `log:"pos:9,color:magenta,keyname:Status"`
	Code         string  `log:"pos:10,color:magenta"`
	Error        string  `log:"pos:11,color:red"`
	UsedTime     float32 `log:"pos:12,color:green,precision:6"`
	Stream       bool    `log:"pos:13"`
	ResponseSize int64   `log:"pos:14,color:magenta,keyname:Size"`
	RequestData  any     `log:"pos:15,color:cyan,keyname:Request"`
	ResponseData any     `log:"pos:16,color:magenta,keyname:Response"`
}

func (entry *CallLog) Reset() {
	entry.BaseLog.Reset()
	entry.Action = ""
	entry.Method = ""
	entry.URL = ""
	entry.StatusCode = 0
	entry.Code = ""
	entry.Error = ""
	entry.UsedTime = 0
	entry.Stream = false
	entry.ResponseSize = 0
	entry.RequestData = nil
	entry.ResponseData = nil
}

func init() {
	log.RegisterType(logTypeAPICall, &CallLog{})
}

func logCall(options *CallOptions, config map[string]any, action, method, url string, request any, result *Result, started time.Time) {
	logger := options.Logger
	if logger == nil {
		logger = log.DefaultLogger
	}
	if logger == nil || !logger.CheckLevel(log.INFO) {
		return
	}
	entry := log.GetEntry[CallLog]()
	logger.FillBase(entry.GetBaseLog(), logTypeAPICall)
	entry.Action = action
	entry.Method = method
	entry.URL = sanitizeLogURL(url, config)
	entry.StatusCode = result.StatusCode
	entry.Code = result.Code
	entry.Error = result.Error
	entry.UsedTime = float32(time.Since(started).Seconds())
	entry.Stream = options.Stream != nil
	entry.ResponseSize = result.receivedBytes
	logging, _ := config["logging"].(map[string]any)
	if cast.Bool(logging["request"]) {
		entry.RequestData = logSnapshot(request, logging)
	}
	if cast.Bool(logging["response"]) {
		entry.ResponseData = logSnapshot(result.Data, logging)
	}
	logger.Log(entry)
}

func sanitizeLogURL(raw string, config map[string]any) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	configured, _ := config["query"].(map[string]any)
	query := parsed.Query()
	changed := false
	for key, value := range configured {
		if _, ok := value.(*safe.SecretPlaintext); ok {
			query.Set(key, "***")
			changed = true
		}
	}
	if changed {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func logSnapshot(value any, config map[string]any) any {
	maxText := cast.Int(config["maxTextLength"])
	if maxText <= 0 {
		maxText = 4096
	}
	maxItems := cast.Int(config["maxArrayItems"])
	if maxItems <= 0 {
		maxItems = 3
	}
	maxDepth := cast.Int(config["maxDepth"])
	if maxDepth <= 0 {
		maxDepth = 4
	}
	return trimLogValue(reflect.ValueOf(value), 0, maxDepth, maxItems, maxText)
}

func trimLogValue(value reflect.Value, depth, maxDepth, maxItems, maxText int) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if depth >= maxDepth {
		return "[max depth]"
	}
	switch value.Kind() {
	case reflect.String:
		text := value.String()
		if len(text) > maxText {
			return text[:maxText] + fmt.Sprintf("...[truncated %d bytes]", len(text)-maxText)
		}
		return text
	case reflect.Map:
		out := map[string]any{}
		iterator := value.MapRange()
		for iterator.Next() {
			out[cast.String(iterator.Key().Interface())] = trimLogValue(iterator.Value(), depth+1, maxDepth, maxItems, maxText)
		}
		return out
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("[binary %d bytes]", value.Len())
		}
		limit := value.Len()
		if limit > maxItems {
			limit = maxItems
		}
		out := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			out = append(out, trimLogValue(value.Index(i), depth+1, maxDepth, maxItems, maxText))
		}
		if value.Len() > limit {
			out = append(out, fmt.Sprintf("[%d more items]", value.Len()-limit))
		}
		return out
	case reflect.Struct:
		out := map[string]any{}
		typeOf := value.Type()
		for i := 0; i < value.NumField(); i++ {
			if typeOf.Field(i).PkgPath == "" {
				out[cast.GetLowerName(typeOf.Field(i).Name)] = trimLogValue(value.Field(i), depth+1, maxDepth, maxItems, maxText)
			}
		}
		return out
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
