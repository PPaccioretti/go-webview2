package edge

import (
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NavigationEvent says at which point a Navigation was intercepted.
type NavigationEvent int

const (
	// NavigationEventStarting: a top-level navigation, or one hop of its
	// redirect chain, is about to start. Refusing it cancels the navigation
	// and the current document stays.
	NavigationEventStarting NavigationEvent = iota
	// NavigationEventDocumentRequest: the network request of a document (a
	// top-level page or a frame) is about to be sent. Refusing it answers the
	// request locally with a 403, so nothing reaches the network. This is
	// needed on top of NavigationEventStarting: WebView2 sends the request of
	// a navigation even when NavigationStarting cancels it, and a URI can
	// carry data out.
	NavigationEventDocumentRequest
	// NavigationEventNewWindow: window.open, a target=_blank link or the like.
	// Refusing it opens no window. A popup is a WebView2 of its own, which the
	// other two checks never see.
	NavigationEventNewWindow
)

// Navigation is what Chromium.NavigationFilter is asked about.
type Navigation struct {
	URI             string
	Event           NavigationEvent
	IsUserInitiated bool
	// IsRedirected is true for the hops of an HTTP redirect, which raise
	// NavigationEventStarting again.
	IsRedirected bool
}

func hresultFailed(hr uintptr) bool {
	return int32(hr) < 0
}

// hresultError reports a failed COM call. This file judges a call by the
// HRESULT it returns, not by the thread's last error the rest of the package
// compares: a COM method may succeed and leave the last error set, or fail
// without touching it.
func hresultError(method string, hr uintptr) error {
	return fmt.Errorf("%s failed: HRESULT 0x%08x", method, uint32(hr))
}

// registerNavigationFilter subscribes what NavigationFilter needs. It runs in
// CreateCoreWebView2ControllerCompleted, before Embed returns, so nothing can
// navigate ahead of it; the first failure is kept and makes Embed fail.
func (e *Chromium) registerNavigationFilter() {
	// Without the WebResourceRequested subscription the document
	// filter below would route requests to no handler: no 403 layer at all.
	if hresultFailed(e.webResourceRequestedHR) {
		e.navigationFilterErr = hresultError("ICoreWebView2.add_WebResourceRequested", e.webResourceRequestedHR)
		return
	}
	var token _EventRegistrationToken
	if hr, _, _ := e.webview.vtbl.AddNavigationStarting.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.navigationStarting)),
		uintptr(unsafe.Pointer(&token)),
	); hresultFailed(hr) {
		e.navigationFilterErr = hresultError("ICoreWebView2.add_NavigationStarting", hr)
		return
	}
	if hr, _, _ := e.webview.vtbl.AddNewWindowRequested.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(e.newWindowRequested)),
		uintptr(unsafe.Pointer(&token)),
	); hresultFailed(hr) {
		e.navigationFilterErr = hresultError("ICoreWebView2.add_NewWindowRequested", hr)
		return
	}
	// The WebResourceRequested handler itself is subscribed by
	// CreateCoreWebView2ControllerCompleted; this adds the filter that routes
	// every document request to it.
	filter, err := windows.UTF16PtrFromString("*")
	if err != nil {
		e.navigationFilterErr = err
		return
	}
	if hr, _, _ := e.webview.vtbl.AddWebResourceRequestedFilter.Call(
		uintptr(unsafe.Pointer(e.webview)),
		uintptr(unsafe.Pointer(filter)),
		uintptr(COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT),
	); hresultFailed(hr) {
		e.navigationFilterErr = hresultError("ICoreWebView2.AddWebResourceRequestedFilter", hr)
	}
}

// filterNavigationStarting cancels whatever NavigationFilter does not allow,
// including a navigation whose URI cannot be read.
func (e *Chromium) filterNavigationStarting(args *ICoreWebView2NavigationStartingEventArgs) {
	allow := false
	if uri, err := args.GetUri(); err == nil {
		userInitiated, _ := args.GetIsUserInitiated()
		redirected, _ := args.GetIsRedirected()
		allow = e.NavigationFilter(Navigation{
			URI:             uri,
			Event:           NavigationEventStarting,
			IsUserInitiated: userInitiated,
			IsRedirected:    redirected,
		})
	} else {
		log.Printf("NavigationStarting: %v", err)
	}
	if !allow {
		if err := args.PutCancel(true); err != nil {
			log.Printf("NavigationStarting: %v", err)
		}
	}
}

// filterNewWindow suppresses every popup NavigationFilter does not allow.
func (e *Chromium) filterNewWindow(args *ICoreWebView2NewWindowRequestedEventArgs) {
	allow := false
	if uri, err := args.GetUri(); err == nil {
		userInitiated, _ := args.GetIsUserInitiated()
		allow = e.NavigationFilter(Navigation{
			URI:             uri,
			Event:           NavigationEventNewWindow,
			IsUserInitiated: userInitiated,
		})
	} else {
		log.Printf("NewWindowRequested: %v", err)
	}
	if !allow {
		if err := args.PutHandled(true); err != nil {
			log.Printf("NewWindowRequested: %v", err)
		}
	}
}

// documentRequestRefused answers a document request NavigationFilter does not
// allow with a local 403 and reports whether it refused it. Requests of any
// other resource context are left alone: other filters may route them here.
//
// The COM calls are made here rather than through the existing helpers
// (GetRequest, GetUri, CreateWebResourceResponse, PutResponse), which judge
// success by the thread's last error instead of the HRESULT (see
// hresultError): either mistake there would let a refused request through or
// crash the host. A request that cannot be read is refused.
func (e *Chromium) documentRequestRefused(args *ICoreWebView2WebResourceRequestedEventArgs) bool {
	var ctx COREWEBVIEW2_WEB_RESOURCE_CONTEXT
	if hr, _, _ := args.vtbl.GetResourceContext.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&ctx)),
	); !hresultFailed(hr) && ctx != COREWEBVIEW2_WEB_RESOURCE_CONTEXT_DOCUMENT {
		return false
	}
	if uri, ok := requestURI(args); ok && e.NavigationFilter(Navigation{URI: uri, Event: NavigationEventDocumentRequest}) {
		return false
	}
	if err := e.respondForbidden(args); err != nil {
		// Nothing else can stop the request from here; say so loudly.
		log.Printf("WebResourceRequested: a refused document request could NOT be blocked and goes out: %v", err)
	}
	return true
}

// requestURI reads the URI of the request behind args, releasing the request.
func requestURI(args *ICoreWebView2WebResourceRequestedEventArgs) (string, bool) {
	var req *ICoreWebView2WebResourceRequest
	if hr, _, _ := args.vtbl.GetRequest.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&req)),
	); hresultFailed(hr) || req == nil {
		return "", false
	}
	defer func() { _, _, _ = req.vtbl.Release.Call(uintptr(unsafe.Pointer(req))) }()
	var uri *uint16
	hr, _, _ := req.vtbl.GetUri.Call(
		uintptr(unsafe.Pointer(req)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hresultFailed(hr) || uri == nil {
		return "", false
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(uri))
	return windows.UTF16PtrToString(uri), true
}

// respondForbidden completes the request with an empty 403 of our own.
func (e *Chromium) respondForbidden(args *ICoreWebView2WebResourceRequestedEventArgs) error {
	reason, err := windows.UTF16PtrFromString("Forbidden")
	if err != nil {
		return err
	}
	headers, err := windows.UTF16PtrFromString("")
	if err != nil {
		return err
	}
	var response *ICoreWebView2WebResourceResponse
	if hr, _, _ := e.environment.vtbl.CreateWebResourceResponse.Call(
		uintptr(unsafe.Pointer(e.environment)),
		0, // no content stream: an empty body
		403,
		uintptr(unsafe.Pointer(reason)),
		uintptr(unsafe.Pointer(headers)),
		uintptr(unsafe.Pointer(&response)),
	); hresultFailed(hr) {
		return hresultError("ICoreWebView2Environment.CreateWebResourceResponse", hr)
	}
	// put_Response takes its own reference; ours is released either way.
	defer func() { _, _, _ = response.vtbl.Release.Call(uintptr(unsafe.Pointer(response))) }()
	if hr, _, _ := args.vtbl.PutResponse.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(response)),
	); hresultFailed(hr) {
		return hresultError("ICoreWebView2WebResourceRequestedEventArgs.put_Response", hr)
	}
	return nil
}

// messageSourceAllowed asks MessageSourceFilter about the document that posted
// the message. A source that cannot be read is refused.
func (e *Chromium) messageSourceAllowed(args *iCoreWebView2WebMessageReceivedEventArgs) bool {
	var source *uint16
	hr, _, _ := args.vtbl.GetSource.Call(
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(&source)),
	)
	if hresultFailed(hr) || source == nil {
		return false
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(source))
	return e.MessageSourceFilter(windows.UTF16PtrToString(source))
}
