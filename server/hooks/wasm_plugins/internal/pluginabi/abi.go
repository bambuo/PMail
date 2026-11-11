//go:build wasip1

// Package pluginabi 封装 WASM 插件与 PMail 宿主之间的内存与 RPC 约定。
package pluginabi

import "unsafe"

//go:wasmimport env pmail_emit
func emit(ptr, size uint32) uint32

//go:wasmimport env pmail_log
func logFn(level, ptr, size uint32)

//go:wasmimport env pmail_http_get
func httpGet(urlPtr, urlLen, outPtr, outCap uint32) int32

//go:wasmimport env pmail_http_post
func httpPost(urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap uint32) int32

var buffers = map[uint32][]byte{}

// Alloc 由宿主调用，申请请求/响应缓冲区。
func Alloc(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

// Free 由宿主调用，释放 Alloc 返回的缓冲区。
func Free(ptr, size uint32) {
	delete(buffers, ptr)
}

// Guest 返回指向宿主传入内存的只读视图。
func Guest(ptr, size uint32) []byte {
	if size == 0 || ptr == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

// Keep 让一段 Go 内存在调用期间保持存活，并返回其 wasm 指针。
func Keep(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	buffers[ptr] = b
	return ptr
}

// Release 释放 Keep 的引用。
func Release(ptr uint32) {
	delete(buffers, ptr)
}

// Emit 把 JSON 结果提交给宿主。
func Emit(b []byte) uint32 {
	ptr := Alloc(uint32(len(b)))
	if ptr == 0 {
		return 1
	}
	copy(Guest(ptr, uint32(len(b))), b)
	code := emit(ptr, uint32(len(b)))
	Free(ptr, uint32(len(b)))
	return code
}

// Log 向宿主日志写入一条消息。
func Log(level uint32, msg string) {
	b := []byte(msg)
	ptr := Keep(b)
	defer Release(ptr)
	logFn(level, ptr, uint32(len(b)))
}

// HTTPGet 发起 GET 请求，返回响应体与状态码（负数为错误）。
func HTTPGet(url string, outCap uint32) ([]byte, int32) {
	urlBytes := []byte(url)
	urlPtr := Keep(urlBytes)
	defer Release(urlPtr)
	outPtr := Alloc(outCap)
	if outPtr == 0 {
		return nil, -1
	}
	defer Free(outPtr, outCap)
	n := httpGet(urlPtr, uint32(len(urlBytes)), outPtr, outCap)
	if n < 0 {
		return nil, n
	}
	return append([]byte(nil), Guest(outPtr, uint32(n))...), n
}

// HTTPPost 发起 JSON POST 请求，返回响应体与状态码（负数为错误）。
func HTTPPost(url string, body []byte, outCap uint32) ([]byte, int32) {
	urlBytes := []byte(url)
	urlPtr := Keep(urlBytes)
	defer Release(urlPtr)
	bodyPtr := Keep(body)
	defer Release(bodyPtr)
	outPtr := Alloc(outCap)
	if outPtr == 0 {
		return nil, -1
	}
	defer Free(outPtr, outCap)
	n := httpPost(urlPtr, uint32(len(urlBytes)), bodyPtr, uint32(len(body)), outPtr, outCap)
	if n < 0 {
		return nil, n
	}
	return append([]byte(nil), Guest(outPtr, uint32(n))...), n
}
