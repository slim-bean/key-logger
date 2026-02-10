package screencap

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics

#include <CoreGraphics/CoreGraphics.h>
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

// CGWindowListCreateImage is obsoleted in the macOS 15 SDK headers but is
// still present in the CoreGraphics dylib. Load it at runtime via dlsym
// to bypass the SDK availability check.
typedef CGImageRef (*CGWindowListCreateImageFunc)(
	CGRect, CGWindowListOption, CGWindowID, CGWindowImageOption);

static CGImageRef createScreenImage(CGRect rect) {
	static CGWindowListCreateImageFunc fn = NULL;
	if (!fn) {
		fn = (CGWindowListCreateImageFunc)dlsym(RTLD_DEFAULT, "CGWindowListCreateImage");
		if (!fn) return NULL;
	}
	return fn(rect, kCGWindowListOptionOnScreenOnly, kCGNullWindowID, kCGWindowImageDefault);
}

static int captureScreenRect(double x, double y, double w, double h,
	unsigned char **outData, int *outWidth, int *outHeight, int *outBytesPerRow) {

	CGRect rect = CGRectMake(x, y, w, h);
	CGImageRef image = createScreenImage(rect);
	if (!image) return 1;

	*outWidth = (int)CGImageGetWidth(image);
	*outHeight = (int)CGImageGetHeight(image);
	*outBytesPerRow = (int)CGImageGetBytesPerRow(image);

	CFDataRef dataRef = CGDataProviderCopyData(CGImageGetDataProvider(image));
	if (!dataRef) {
		CGImageRelease(image);
		return 2;
	}

	CFIndex length = CFDataGetLength(dataRef);
	*outData = (unsigned char *)malloc(length);
	if (!*outData) {
		CFRelease(dataRef);
		CGImageRelease(image);
		return 3;
	}

	memcpy(*outData, CFDataGetBytePtr(dataRef), length);
	CFRelease(dataRef);
	CGImageRelease(image);
	return 0;
}

static void freeData(unsigned char *p) { free(p); }
*/
import "C"

import (
	"fmt"
	"image"
	"unsafe"
)

type darwinCapturer struct{}

// New creates a new Capturer for macOS using CGWindowListCreateImage.
// Requires Screen Recording permission in System Settings.
func New() Capturer {
	return &darwinCapturer{}
}

func (c *darwinCapturer) Init() error {
	return nil
}

func (c *darwinCapturer) CaptureRect(bounds image.Rectangle) (*image.RGBA, error) {
	var data *C.uchar
	var width, height, bytesPerRow C.int

	ret := C.captureScreenRect(
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		&data, &width, &height, &bytesPerRow,
	)
	if ret != 0 {
		return nil, fmt.Errorf("screen capture failed (code %d); ensure Screen Recording permission is granted in System Settings > Privacy & Security > Screen Recording", ret)
	}
	defer C.freeData(data)

	w := int(width)
	h := int(height)
	bpr := int(bytesPerRow)

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// CoreGraphics returns pixel data in BGRA byte order
	// (premultiplied-first, little-endian 32-bit).
	// Convert BGRA -> RGBA for Go's image.RGBA format.
	src := unsafe.Slice((*byte)(unsafe.Pointer(data)), h*bpr)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := y*bpr + x*4
			di := y*img.Stride + x*4
			img.Pix[di+0] = src[si+2] // R
			img.Pix[di+1] = src[si+1] // G
			img.Pix[di+2] = src[si+0] // B
			img.Pix[di+3] = 255       // A (force opaque)
		}
	}

	return img, nil
}
