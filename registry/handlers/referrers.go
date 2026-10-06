package handlers

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/internal/dcontext"
	"github.com/distribution/distribution/v3/manifest/ocischema"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func referrersDispatcher(ctx *Context, r *http.Request) http.Handler {
	dgst, err := getDigest(ctx)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx.Errors = append(ctx.Errors, errcode.ErrorCodeDigestInvalid.WithDetail(err))
		})
	}

	referrersHandler := &referrersHandler{
		Context: ctx,
		Digest:  dgst,
	}

	return handlers.MethodHandler{
		http.MethodGet: http.HandlerFunc(referrersHandler.GetReferrers),
	}
}

type referrersHandler struct {
	*Context

	Digest digest.Digest
}

func (rh *referrersHandler) GetReferrers(w http.ResponseWriter, r *http.Request) {
	dcontext.GetLogger(rh).Debug("GetReferrers")

	manifests, err := rh.Repository.Manifests(rh)
	if err != nil {
		rh.Errors = append(rh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

	enumerator, ok := manifests.(distribution.ManifestEnumerator)
	if !ok {
		rh.Errors = append(rh.Errors, errcode.ErrorCodeUnsupported)
		return
	}

	artifactTypeFilter := r.URL.Query().Get("artifactType")
	seen := map[digest.Digest]struct{}{}
	referrers := make([]v1.Descriptor, 0)

	err = enumerator.Enumerate(rh, func(dgst digest.Digest) error {
		if _, ok := seen[dgst]; ok {
			return nil
		}
		seen[dgst] = struct{}{}

		manifest, err := manifests.Get(rh, dgst)
		if err != nil {
			if _, ok := err.(distribution.ErrManifestUnknownRevision); ok {
				return nil
			}
			return err
		}

		desc, subject, ok, err := describeReferrer(manifest, dgst)
		if err != nil {
			return err
		}
		if !ok || subject == nil || subject.Digest != rh.Digest {
			return nil
		}
		if artifactTypeFilter != "" && desc.ArtifactType != artifactTypeFilter {
			return nil
		}

		referrers = append(referrers, desc)
		return nil
	})
	if err != nil {
		if _, ok := err.(driver.PathNotFoundError); ok {
			err = nil
		} else {
			rh.Errors = append(rh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	}

	slices.SortFunc(referrers, func(a, b v1.Descriptor) int {
		return compareStrings(a.Digest.String(), b.Digest.String())
	})

	index, err := ocischema.FromDescriptors(referrers, nil)
	if err != nil {
		rh.Errors = append(rh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

	mediaType, payload, err := index.Payload()
	if err != nil {
		rh.Errors = append(rh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
	if artifactTypeFilter != "" {
		w.Header().Set("OCI-Filters-Applied", "artifactType")
	}
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(payload); err != nil {
		dcontext.GetLogger(rh).WithError(err).Error("error writing referrers response")
	}
}

func describeReferrer(manifest distribution.Manifest, dgst digest.Digest) (v1.Descriptor, *v1.Descriptor, bool, error) {
	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return v1.Descriptor{}, nil, false, err
	}

	desc := v1.Descriptor{
		MediaType: mediaType,
		Digest:    dgst,
		Size:      int64(len(payload)),
	}

	switch m := manifest.(type) {
	case *ocischema.DeserializedManifest:
		desc.Annotations = m.Annotations
		if m.ArtifactType != "" {
			desc.ArtifactType = m.ArtifactType
		} else if m.Config.MediaType != "" {
			desc.ArtifactType = m.Config.MediaType
		}
		return desc, m.Subject, true, nil
	case *ocischema.DeserializedImageIndex:
		desc.Annotations = m.Annotations
		if m.ArtifactType != "" {
			desc.ArtifactType = m.ArtifactType
		}
		return desc, m.Subject, true, nil
	default:
		return v1.Descriptor{}, nil, false, nil
	}
}

func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
