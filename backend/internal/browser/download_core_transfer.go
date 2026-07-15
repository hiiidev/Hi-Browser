package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func doConcurrentDownload(ctx context.Context, client *http.Client, targetUrl string, tempFile *os.File, sendEvent func(string, int, string), statsCallbacks ...func(int64, int64, int64)) error {
	info, err := tempFile.Stat()
	if err != nil {
		return err
	}
	offset := info.Size()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetUrl, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if offset > 0 && resp.StatusCode == http.StatusOK {
		if err := tempFile.Truncate(0); err != nil {
			return err
		}
		if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		offset = 0
		sendEvent("downloading", 0, "服务器不支持 Range，已回退为单连接重新下载")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HTTP状态码异常: %d", resp.StatusCode)
	}
	total := resp.ContentLength + offset
	if value := resp.Header.Get("Content-Range"); value != "" {
		if slash := strings.LastIndex(value, "/"); slash >= 0 {
			if parsed, e := strconv.ParseInt(value[slash+1:], 10, 64); e == nil {
				total = parsed
			}
		}
	}
	if total <= 0 {
		return fmt.Errorf("服务器返回了异常的 Content-Length")
	}
	if _, err := tempFile.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	start := time.Now()
	downloaded := offset
	lastTick := time.Time{}
	writer := &coreDownloadWriter{ctx: ctx, writeFunc: func(p []byte) (int, error) {
		n, e := tempFile.Write(p)
		downloaded += int64(n)
		if time.Since(lastTick) >= time.Second {
			progress := int(float64(downloaded) * 100 / float64(total))
			elapsed := time.Since(start).Seconds()
			speed := int64(0)
			if elapsed > 0 {
				speed = int64(float64(downloaded-offset) / elapsed)
			}
			for _, callback := range statsCallbacks {
				if callback != nil {
					callback(downloaded, total, speed)
				}
			}
			sendEvent("downloading", progress, fmt.Sprintf("下载中 %.2f / %.2f MB，%.2f MB/s", float64(downloaded)/1048576, float64(total)/1048576, float64(speed)/1048576))
			lastTick = time.Now()
		}
		return n, e
	}}
	_, err = io.CopyBuffer(writer, resp.Body, make([]byte, 1024*1024))
	if err != nil {
		return err
	}
	if downloaded != total {
		return fmt.Errorf("下载长度异常：预期 %d 字节，实际 %d 字节", total, downloaded)
	}
	return nil
}

func doSingleThreadDownload(ctx context.Context, client *http.Client, targetUrl string, tempFile *os.File, totalSize int64, sendEvent func(string, int, string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetUrl, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码异常: %d", resp.StatusCode)
	}

	var downloaded int64
	var lastTick time.Time

	pw := &coreDownloadWriter{
		writeFunc: func(p []byte) (n int, err error) {
			n, err = tempFile.Write(p)
			if n > 0 {
				downloaded += int64(n)
				if totalSize > 0 && time.Since(lastTick) > time.Second {
					percent := int((float64(downloaded) / float64(totalSize)) * 100)
					sendEvent("downloading", percent, fmt.Sprintf("单流下载中... %.2f MB / %.2f MB", float64(downloaded)/1024/1024, float64(totalSize)/1024/1024))
					lastTick = time.Now()
				}
			}
			return n, err
		},
		ctx: ctx,
	}

	buf := make([]byte, 1024*1024)
	_, err = io.CopyBuffer(pw, resp.Body, buf)
	return err
}
