// handlers.go
package goHttpHandler

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	minGzipSize = 1024
)

var (
	gzipableContents = []string{"text/plain", "text/html", "text/css", "application/json", "application/javascript",
		"application/x-javascript", "text/xml", "application/xml", "application/xml+rss", "text/javascript"}
)

type statWriter struct {
	writer io.Writer
	bytes  int
}

func (sw *statWriter) Write(data []byte) (int, error) {
	n, err := sw.writer.Write(data)
	if err == nil {
		sw.bytes += n
	}
	return n, err
}

type gzipRespWriter struct {
	writer        http.ResponseWriter
	request       *http.Request
	sw            *statWriter
	gzipWriter    *gzip.Writer
	written       bool
	headerSet     bool
	gzipContent   bool
	statusCode    int
	origBytes     int
	contentLength int
}

func newGzipRespWriter(w http.ResponseWriter, r *http.Request) *gzipRespWriter {
	result := &gzipRespWriter{
		writer:     w,
		request:    r,
		statusCode: http.StatusOK,
	}

	return result
}

func (grw *gzipRespWriter) Header() http.Header {
	return grw.writer.Header()
}

func (grw *gzipRespWriter) Write(data []byte) (int, error) {
	if h := grw.writer.Header(); h.Get("Content-Type") == "" {
		h.Set("Content-Type", http.DetectContentType(data))
	}

	grw.checkContentLength()

	if !grw.written {
		grw.gzipContent = grw.canGzip(len(data))
		if grw.gzipContent {
			grw.setHeader()
		}
	}

	grw.written = true
	grw.origBytes += len(data)

	if grw.gzipContent {
		if grw.sw == nil {
			grw.sw = &statWriter{writer: grw.writer}
			grw.gzipWriter = gzip.NewWriter(grw.sw)
		}
		n, err := grw.gzipWriter.Write(data)
		if grw.origBytes == grw.contentLength || err != nil {
			grw.finish()
		}
		return n, err
	}

	return grw.writer.Write(data)
}

func (grw *gzipRespWriter) WriteHeader(statusCode int) {
	grw.statusCode = statusCode
	grw.checkContentLength()
	grw.writer.WriteHeader(statusCode)
}

func (grw *gzipRespWriter) finish() {
	if grw.gzipWriter == nil {
		return
	}

	grw.setHeader()
	grw.gzipWriter.Close()
	grw.gzipWriter = nil
}

func (grw *gzipRespWriter) getBytesWritten() int {
	if grw.gzipContent {
		return grw.sw.bytes
	} else {
		return grw.origBytes
	}
}

func (grw *gzipRespWriter) checkContentLength() {
	if grw.contentLength > 0 || grw.written {
		return
	}

	if cl := grw.Header().Get("Content-Length"); len(cl) > 0 {
		if t, e := strconv.ParseInt(cl, 10, 32); e == nil {
			grw.contentLength = int(t)
			if grw.canGzip(grw.origBytes) {
				grw.setHeader()
			}
		}
	}
}

func (grw *gzipRespWriter) setHeader() {
	if grw.headerSet {
		return
	}

	grw.writer.Header().Set("Content-Encoding", "gzip")
	grw.writer.Header().Del("Content-Length")
	grw.headerSet = true
}

func (grw *gzipRespWriter) canGzip(length int) bool {
	acceptGzip := false
	for _, encoding := range strings.Split(grw.request.Header.Get("Accept-Encoding"), ",") {
		if "gzip" == strings.TrimSpace(encoding) {
			acceptGzip = true
			break
		}
	}

	if !acceptGzip || grw.statusCode != http.StatusOK {
		return false
	}

	gzipable := false
	ct := grw.writer.Header().Get("Content-Type")
	for _, gzipCt := range gzipableContents {
		if ct == gzipCt || strings.HasPrefix(ct, gzipCt+"; ") {
			gzipable = true
			break
		}
	}

	if !gzipable {
		return false
	}

	return length > minGzipSize || grw.contentLength > minGzipSize
}

type HttpLogGzipHandler struct {
	impl      http.Handler
	logWriter io.Writer
}

func NewHttpLogGzipHandler(handler http.Handler, writer io.Writer) *HttpLogGzipHandler {
	return &HttpLogGzipHandler{
		impl:      handler,
		logWriter: writer,
	}
}

func (handler *HttpLogGzipHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start, gzipWriter := time.Now(), newGzipRespWriter(w, r)
	handler.impl.ServeHTTP(gzipWriter, r)
	gzipWriter.finish()
	millis := time.Now().Sub(start).Nanoseconds() / 1000000

	contentType := w.Header().Get("Content-Type")

	referer, ip := r.Referer(), r.RemoteAddr[:strings.LastIndex(r.RemoteAddr, ":")]
	if len(referer) == 0 {
		referer = "-"
	}

	handler.logWriter.Write([]byte(fmt.Sprintf("%s %s %s %d %d/%d %dms %s '%s' %s",
		ip, r.Method, r.RequestURI, gzipWriter.statusCode, gzipWriter.getBytesWritten(),
		gzipWriter.origBytes, millis, referer, contentType, r.UserAgent())))
}
