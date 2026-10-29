package edge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// _ICoreWebView2NavigationStartingEventArgsVtbl mirrors the declaration order
// of ICoreWebView2NavigationStartingEventArgs in WebView2.h. COM slots are
// positional: reordering a field silently calls a different method.
type _ICoreWebView2NavigationStartingEventArgsVtbl struct {
	_IUnknownVtbl
	GetUri             ComProc
	GetIsUserInitiated ComProc
	GetIsRedirected    ComProc
	GetRequestHeaders  ComProc
	GetCancel          ComProc
	PutCancel          ComProc
	GetNavigationId    ComProc
}

type ICoreWebView2NavigationStartingEventArgs struct {
	vtbl *_ICoreWebView2NavigationStartingEventArgsVtbl
}

// GetUri returns the URI the navigation is about to load.
func (i *ICoreWebView2NavigationStartingEventArgs) GetUri() (string, error) {
	var uri *uint16
	hr, _, _ := i.vtbl.GetUri.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hresultFailed(hr) {
		return "", hresultError("ICoreWebView2NavigationStartingEventArgs.get_Uri", hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(uri))
	return windows.UTF16PtrToString(uri), nil
}

// GetIsUserInitiated reports whether the navigation came from a user gesture.
func (i *ICoreWebView2NavigationStartingEventArgs) GetIsUserInitiated() (bool, error) {
	var v int32
	hr, _, _ := i.vtbl.GetIsUserInitiated.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&v)),
	)
	if hresultFailed(hr) {
		return false, hresultError("ICoreWebView2NavigationStartingEventArgs.get_IsUserInitiated", hr)
	}
	return v != 0, nil
}

// GetIsRedirected reports whether this event is a redirect hop of a navigation
// already started: WebView2 raises NavigationStarting again for each one.
func (i *ICoreWebView2NavigationStartingEventArgs) GetIsRedirected() (bool, error) {
	var v int32
	hr, _, _ := i.vtbl.GetIsRedirected.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&v)),
	)
	if hresultFailed(hr) {
		return false, hresultError("ICoreWebView2NavigationStartingEventArgs.get_IsRedirected", hr)
	}
	return v != 0, nil
}

// GetCancel reports whether the navigation has been cancelled.
func (i *ICoreWebView2NavigationStartingEventArgs) GetCancel() (bool, error) {
	var v int32
	hr, _, _ := i.vtbl.GetCancel.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&v)),
	)
	if hresultFailed(hr) {
		return false, hresultError("ICoreWebView2NavigationStartingEventArgs.get_Cancel", hr)
	}
	return v != 0, nil
}

// PutCancel cancels the navigation when called with true from the event
// handler. The current document stays loaded.
func (i *ICoreWebView2NavigationStartingEventArgs) PutCancel(cancel bool) error {
	hr, _, _ := i.vtbl.PutCancel.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(boolToInt(cancel)),
	)
	if hresultFailed(hr) {
		return hresultError("ICoreWebView2NavigationStartingEventArgs.put_Cancel", hr)
	}
	return nil
}

// GetNavigationId returns the id shared by every event of this navigation,
// redirect hops and NavigationCompleted included.
func (i *ICoreWebView2NavigationStartingEventArgs) GetNavigationId() (uint64, error) {
	var id uint64
	hr, _, _ := i.vtbl.GetNavigationId.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&id)),
	)
	if hresultFailed(hr) {
		return 0, hresultError("ICoreWebView2NavigationStartingEventArgs.get_NavigationId", hr)
	}
	return id, nil
}
