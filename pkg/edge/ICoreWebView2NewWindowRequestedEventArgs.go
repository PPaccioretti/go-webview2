package edge

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// _ICoreWebView2NewWindowRequestedEventArgsVtbl mirrors the declaration order
// of ICoreWebView2NewWindowRequestedEventArgs in WebView2.h. COM slots are
// positional: reordering a field silently calls a different method.
type _ICoreWebView2NewWindowRequestedEventArgsVtbl struct {
	_IUnknownVtbl
	GetUri             ComProc
	PutNewWindow       ComProc
	GetNewWindow       ComProc
	PutHandled         ComProc
	GetHandled         ComProc
	GetIsUserInitiated ComProc
	GetDeferral        ComProc
	GetWindowFeatures  ComProc
}

type ICoreWebView2NewWindowRequestedEventArgs struct {
	vtbl *_ICoreWebView2NewWindowRequestedEventArgsVtbl
}

// GetUri returns the URI the new window would load.
func (i *ICoreWebView2NewWindowRequestedEventArgs) GetUri() (string, error) {
	var uri *uint16
	hr, _, _ := i.vtbl.GetUri.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hresultFailed(hr) {
		return "", hresultError("ICoreWebView2NewWindowRequestedEventArgs.get_Uri", hr)
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(uri))
	return windows.UTF16PtrToString(uri), nil
}

// PutHandled with true and no NewWindow set tells WebView2 not to open any
// window: window.open gets a WindowProxy that loads nothing.
func (i *ICoreWebView2NewWindowRequestedEventArgs) PutHandled(handled bool) error {
	hr, _, _ := i.vtbl.PutHandled.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(boolToInt(handled)),
	)
	if hresultFailed(hr) {
		return hresultError("ICoreWebView2NewWindowRequestedEventArgs.put_Handled", hr)
	}
	return nil
}

// GetIsUserInitiated reports whether the request came from a user gesture.
func (i *ICoreWebView2NewWindowRequestedEventArgs) GetIsUserInitiated() (bool, error) {
	var v int32
	hr, _, _ := i.vtbl.GetIsUserInitiated.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&v)),
	)
	if hresultFailed(hr) {
		return false, hresultError("ICoreWebView2NewWindowRequestedEventArgs.get_IsUserInitiated", hr)
	}
	return v != 0, nil
}
