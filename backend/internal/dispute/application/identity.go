package application

import (
	"context"
	"slices"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// MaxImageBytes caps a logo or an avatar: these are shown at 40 pixels, so anything larger is a mistake.
const MaxImageBytes = 256 << 10

// AllowedImageTypes leaves out SVG on purpose: it is a document that can carry script, and it would be served
// from the API's own origin.
var AllowedImageTypes = []string{"image/png", "image/jpeg", "image/webp"}

// ErrImageRefused is an image the workbench will not store.
var ErrImageRefused = errs.New(errs.Unprocessable, "image-refused", "the image cannot be stored")

// Image is a stored picture with the type it was uploaded as.
type Image struct {
	Content     []byte
	ContentType string
}

// TenantProfile is what the workbench shows about an organisation: its name, and whether it has a logo to draw.
type TenantProfile struct {
	Name          string
	HasLogo       bool
	LogoUpdatedAt *time.Time
}

func checkImage(img Image) error {
	switch {
	case len(img.Content) == 0:
		return ErrImageRefused.WithDetail("the file is empty")
	case len(img.Content) > MaxImageBytes:
		return ErrImageRefused.WithDetail("the image is larger than %d kB", MaxImageBytes>>10)
	case !slices.Contains(AllowedImageTypes, img.ContentType):
		return ErrImageRefused.WithDetail("%s is not a PNG, JPEG or WebP", img.ContentType)
	}
	return nil
}

// Tenant reads the organisation behind the current request.
func (s *Service) Tenant(ctx context.Context) (TenantProfile, error) {
	var out TenantProfile
	err := s.store.WithTx(ctx, func(tx Tx) error {
		name, err := tx.TenantName(ctx)
		if err != nil {
			return err
		}
		out.Name = name
		meta, err := tx.GetTenantMetadata(ctx)
		if err != nil {
			return err
		}
		out.HasLogo, out.LogoUpdatedAt = meta.HasLogo, meta.LogoUpdatedAt
		return nil
	})
	return out, err
}

// TenantLogo returns the organisation's logo, or ErrNotFound when it has none.
func (s *Service) TenantLogo(ctx context.Context) (Image, error) {
	var out Image
	err := s.store.WithTx(ctx, func(tx Tx) error {
		img, err := tx.GetTenantLogo(ctx)
		out = img
		return err
	})
	return out, err
}

// PutTenantLogo replaces the organisation's logo and answers with the profile the workbench will redraw from.
func (s *Service) PutTenantLogo(ctx context.Context, img Image) (TenantProfile, error) {
	if err := checkImage(img); err != nil {
		return TenantProfile{}, err
	}
	var out TenantProfile
	err := s.store.WithTx(ctx, func(tx Tx) error {
		now := s.now()
		if err := tx.PutTenantLogo(ctx, img, now); err != nil {
			return err
		}
		name, err := tx.TenantName(ctx)
		if err != nil {
			return err
		}
		out = TenantProfile{Name: name, HasLogo: true, LogoUpdatedAt: &now}
		return nil
	})
	return out, err
}

// AnalystAvatar returns one analyst's picture, or ErrNotFound when they have not uploaded one.
func (s *Service) AnalystAvatar(ctx context.Context, subject string) (Image, error) {
	var out Image
	err := s.store.WithTx(ctx, func(tx Tx) error {
		img, err := tx.GetAnalystAvatar(ctx, subject)
		out = img
		return err
	})
	return out, err
}

// PutAnalystAvatar replaces one analyst's picture.
func (s *Service) PutAnalystAvatar(ctx context.Context, subject string, img Image) error {
	if err := checkImage(img); err != nil {
		return err
	}
	return s.store.WithTx(ctx, func(tx Tx) error {
		return tx.PutAnalystAvatar(ctx, subject, img, s.now())
	})
}
