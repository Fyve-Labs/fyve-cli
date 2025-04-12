package images

import (
	"fmt"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Image holds information about an image.
type Image struct {
	// Domain is the registry host of this image
	Domain string
	// Path may include username like portainer/portainer-ee, no Tag or Digest
	Path   string
	Tag    string
	Digest digest.Digest
	named  reference.Named
	opts   ParseImageOptions
}

// ParseImageOptions holds image options for parsing.
type ParseImageOptions struct {
	Name   string
	HubTpl string
}

// Name returns the full name representation of an image but no Tag or Digest.
func (i *Image) Name() string {
	return i.named.Name()
}

// FullName return the real full name may include Tag or Digest of the image, Tag first.
func (i *Image) FullName() string {
	if i.Tag == "" {
		return fmt.Sprintf("%s@%s", i.Name(), i.Digest)
	}
	return fmt.Sprintf("%s:%s", i.Name(), i.Tag)
}

// String returns the string representation of an image, including Tag and Digest if existed.
func (i *Image) String() string {
	return i.named.String()
}

// Reference returns either the digest if it is non-empty or the tag for the image.
func (i *Image) Reference() string {
	if len(i.Digest.String()) > 1 {
		return i.Digest.String()
	}

	return i.Tag
}

// WithDigest sets the digest for an image.
func (i *Image) WithDigest(digest digest.Digest) (err error) {
	i.Digest = digest
	i.named, err = reference.WithDigest(i.named, digest)
	return err
}

func (i *Image) WithTag(tag string) (err error) {
	i.Tag = tag
	i.named, err = reference.WithTag(i.named, tag)
	return err
}

func (i *Image) trimDigest() error {
	i.Digest = ""
	named, err := ParseImage(ParseImageOptions{Name: i.FullName()})
	if err != nil {
		return err
	}
	i.named = &named
	return nil
}

// ParseImage returns an Image struct with all the values filled in for a given image.
func ParseImage(parseOpts ParseImageOptions) (Image, error) {
	// Parse the image name and tag.
	named, err := reference.ParseNormalizedNamed(parseOpts.Name)
	if err != nil {
		return Image{}, errors.Wrapf(err, "parsing image %s failed", parseOpts.Name)
	}
	// Add the latest lag if they did not provide one.
	named = reference.TagNameOnly(named)

	i := Image{
		opts:   parseOpts,
		named:  named,
		Domain: reference.Domain(named),
		Path:   reference.Path(named),
	}

	// Add the tag if there was one.
	if tagged, ok := named.(reference.Tagged); ok {
		i.Tag = tagged.Tag()
	}

	// Add the digest if there was one.
	if canonical, ok := named.(reference.Canonical); ok {
		i.Digest = canonical.Digest()
	}

	return i, nil
}
