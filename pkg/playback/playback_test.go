package playback

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_splitKey(t *testing.T) {
	key := "s3.wasabisys.com/xstohj6tqq33.screencap/caps/2023/3/3/1677884068_ExplorelokipersonalGrafanaMozillaFirefox.jpg"
	bucket, obj := splitKey(key)
	assert.Equal(t, "xstohj6tqq33.screencap", bucket)
	assert.Equal(t, "caps/2023/3/3/1677884068_ExplorelokipersonalGrafanaMozillaFirefox.jpg", obj)
}
