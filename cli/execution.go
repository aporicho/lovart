package cli

import "github.com/aporicho/lovart/internal/envelope"

const (
	executionLocal     = "local"
	executionPreflight = "preflight"
	executionSubmit    = "submit"
)

func okLocal(data any, cacheUsed ...bool) envelope.Envelope {
	meta := envelope.ExecutionMetadata{
		ExecutionClass:  executionLocal,
		NetworkRequired: false,
		RemoteWrite:     false,
	}
	if len(cacheUsed) > 0 {
		meta.CacheUsed = boolPtr(cacheUsed[0])
	}
	return envelope.OKWithMetadata(data, meta)
}

func okPreflight(data any, cacheUsed ...bool) envelope.Envelope {
	meta := envelope.ExecutionMetadata{
		ExecutionClass:  executionPreflight,
		NetworkRequired: true,
		RemoteWrite:     false,
	}
	if len(cacheUsed) > 0 {
		meta.CacheUsed = boolPtr(cacheUsed[0])
	}
	return envelope.OKWithMetadata(data, meta)
}

func okSubmit(data any, submitted bool) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  executionSubmit,
		NetworkRequired: true,
		RemoteWrite:     submitted,
		Submitted:       boolPtr(submitted),
	})
}

func okRemoteWrite(data any) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  executionSubmit,
		NetworkRequired: true,
		RemoteWrite:     true,
	})
}

func okPreflightSubmission(data any, submitted bool) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  executionPreflight,
		NetworkRequired: true,
		RemoteWrite:     false,
		Submitted:       boolPtr(submitted),
	})
}

func boolPtr(value bool) *bool {
	return &value
}
