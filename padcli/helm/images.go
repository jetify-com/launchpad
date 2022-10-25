package helm

import (
	"strings"

	"go.jetpack.io/launchpad/padcli/provider"
)

type ImageProvider struct {
	// Should include tag if it exists
	defaultLocalImage string

	// For published images, key is local image, value is published image
	imagePublishMap map[string]string
}

func NewImageProvider(
	defaultLocalImage string,
	imagePublishMap map[string]string,
) *ImageProvider {
	return &ImageProvider{
		defaultLocalImage: defaultLocalImage,
		imagePublishMap:   imagePublishMap,
	}
}

func (i *ImageProvider) get(c provider.Cluster, img string) string {
	if i == nil {
		return img
	}

	if c.IsLocal() {
		if img != "" {
			return img
		}
		return i.defaultLocalImage
	}

	if i.imagePublishMap[img] != "" {
		// We found a published image, use it.
		return i.imagePublishMap[img]
	}

	if img != "" {
		// We have a specified image, assume it's already published and use it.
		return img
	}

	// We have no specified image, use the default published image.
	// TODO: Should we validate?
	return i.imagePublishMap[i.defaultLocalImage]
}

func (i *ImageProvider) getSplit(
	c provider.Cluster,
	img string,
) (string, string) {
	image := i.get(c, img)
	parts := strings.Split(image, ":")
	imageLocation := image
	imageTag := ""
	if len(parts) == 2 {
		imageLocation = parts[0]
		imageTag = parts[1]
	}
	return imageLocation, imageTag
}
