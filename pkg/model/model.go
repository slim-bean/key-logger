package model

import "image"

type Image struct {
	Location string
	Image    *image.RGBA
}
