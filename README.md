# goHttpHandler

A HTTP gzip and log Handler for golang.

You can pass a http.Handler as a real business HTTP handler and an io.Writer as log writer to the constructor, then you get the gzip and log feature.

gzip happens only for:

* client set the Accept-Encoding header and the header includes gzip
* statusCode is 200
* specific mime-types (which depends on http.ResponseWriter.Header().Get("Content-Type")), if you don't set content-type, then http.DetectContentType is called to set the Content-Type header
* content length is bigger than 1KB

the log will log the fields below:

* client ip
* method
* request uri
* status code
* bytes sent to the client (i.e. the gzipped content length)
* bytes of the original content (i.e. the original content length)
* request process time (in milli-seconds)
* referer
* content type
* client user agent

time is not logged and you can do it in your io.Writer