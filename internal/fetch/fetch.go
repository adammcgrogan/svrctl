// Package fetch downloads files while reporting progress.
//
// svrctl's two slow operations — pulling a server jar and pulling a ~200 MB
// JDK — used to print one line and then go silent for minutes, which is
// indistinguishable from a hang. Everything that downloads goes through here
// so the caller can render a real progress bar.
package fetch

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// Client is used for every outbound HTTP request svrctl makes. Without a
// timeout, a stalled upstream — one that accepts the connection but never
// sends a response — hangs the command indefinitely with no way to cancel
// short of killing the process. ResponseHeaderTimeout bounds how long we'll
// wait for that first byte back; once headers arrive, a large download is
// still free to run at whatever speed the connection allows rather than
// being cut off by an overall deadline.
var Client = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

// Progress is a snapshot of a download. Total is 0 when the server did not
// send a Content-Length, in which case only Done is meaningful and the caller
// should fall back to an indeterminate indicator.
type Progress struct {
	Done  int64
	Total int64
}

// Ratio returns completion in [0,1], or 0 when the total size is unknown.
func (p Progress) Ratio() float64 {
	if p.Total <= 0 {
		return 0
	}
	if p.Done >= p.Total {
		return 1
	}
	return float64(p.Done) / float64(p.Total)
}

// Reporter receives progress updates. It is called from the goroutine driving
// the copy, at most once every reportInterval plus once at completion, and may
// be nil.
type Reporter func(Progress)

// reportInterval bounds how often a Reporter is called. Bubble Tea redraws on
// every message, so an unthrottled reader would spend more time rendering
// frames than downloading.
const reportInterval = 80 * time.Millisecond

// Open issues a GET and returns the body along with its advertised size.
// The caller must close the returned reader.
func Open(url string) (io.ReadCloser, int64, error) {
	resp, err := Client.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("downloading %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}
	return resp.Body, resp.ContentLength, nil
}

// Reader wraps r so that every read advances a progress Reporter. Use it when
// the body is streamed into something other than a file, such as an archive
// extractor.
func Reader(r io.Reader, total int64, report Reporter) io.Reader {
	if report == nil {
		return r
	}
	return &progressReader{inner: r, total: total, report: report}
}

// ToFile downloads url into destPath, reporting progress along the way.
func ToFile(url, destPath string, report Reporter) error {
	body, total, err := Open(url)
	if err != nil {
		return err
	}
	defer body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, Reader(body, total, report))
	if err != nil {
		return fmt.Errorf("saving %s: %w", destPath, err)
	}
	if report != nil {
		// Report the bytes actually written, not the advertised length: a
		// chunked response reports -1, and echoing that back would leave the
		// progress bar showing nonsense at the moment it finishes.
		if total <= 0 {
			total = n
		}
		report(Progress{Done: n, Total: total})
	}
	return nil
}

type progressReader struct {
	inner    io.Reader
	total    int64
	done     atomic.Int64
	report   Reporter
	lastSent time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.inner.Read(b)
	if n > 0 {
		done := p.done.Add(int64(n))
		if now := time.Now(); now.Sub(p.lastSent) >= reportInterval {
			p.lastSent = now
			p.report(Progress{Done: done, Total: p.total})
		}
	}
	if err == io.EOF {
		p.report(Progress{Done: p.done.Load(), Total: p.total})
	}
	return n, err
}
