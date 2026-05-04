package signing

import (
	"context"
	"embed"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed assets/*.wasm
var signingAssets embed.FS

// wazeroSigner loads and calls the Lovart WASM signing module.
type wazeroSigner struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	mod      api.Module
	gs       api.Function
	stackPtr api.Function
	wMalloc  api.Function
	wRealloc api.Function
	wFree    api.Function
	memory   api.Memory
}

// NewSigner creates a Signer backed by the embedded Lovart WASM module.
// The WASM is instantiated once and reused across all Sign calls.
func NewSigner() (Signer, error) {
	ctx := context.Background()

	wasmBytes, err := signingAssets.ReadFile("assets/26bd3a5bd74c3c92.wasm")
	if err != nil {
		return nil, fmt.Errorf("signing: read embedded wasm: %w", err)
	}

	rt := wazero.NewRuntime(ctx)

	// Provide empty "wbg" host module (mirrors JS: { wbg: {} }).
	wbgBuilder := rt.NewHostModuleBuilder("wbg")
	wbgBuilder.NewFunctionBuilder().WithFunc(func(_ context.Context, _ api.Module, _ uint32, _ uint32) {
	}).Export("__wbindgen_throw")
	if _, err = wbgBuilder.Instantiate(ctx); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("signing: host wbg module: %w", err)
	}

	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("signing: compile wasm: %w", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		compiled.Close(ctx)
		rt.Close(ctx)
		return nil, fmt.Errorf("signing: instantiate wasm: %w", err)
	}

	s := &wazeroSigner{
		runtime:  rt,
		compiled: compiled,
		mod:      mod,
		gs:        mod.ExportedFunction("gs"),
		stackPtr:  mod.ExportedFunction("__wbindgen_add_to_stack_pointer"),
		wMalloc:   mod.ExportedFunction("__wbindgen_export2"),
		wRealloc:  mod.ExportedFunction("__wbindgen_export3"),
		wFree:     mod.ExportedFunction("__wbindgen_export"),
		memory:    mod.Memory(),
	}

	if s.gs == nil || s.stackPtr == nil || s.wMalloc == nil ||
		s.wRealloc == nil || s.wFree == nil || s.memory == nil {
		mod.Close(ctx)
		compiled.Close(ctx)
		rt.Close(ctx)
		return nil, fmt.Errorf("signing: missing required wasm exports")
	}

	return s, nil
}

// Sign computes a Lovart request signature using the WASM module.
func (s *wazeroSigner) Sign(ctx context.Context, payload SigningPayload) (*SigningResult, error) {
	third := payload.Third
	fourth := payload.Fourth

	// Allocate stack: __wbindgen_add_to_stack_pointer(-16)
	retPtrRaw, err := s.stackPtr.Call(ctx, api.EncodeI32(-16))
	if err != nil {
		return nil, fmt.Errorf("signing: stack alloc: %w", err)
	}
	retPtr := uint32(retPtrRaw[0])

	// Pass 4 strings into WASM memory.
	ptr0, len0, err := s.passString(ctx, payload.Timestamp)
	if err != nil {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: pass timestamp: %w", err)
	}
	defer s.freeBytes(ctx, ptr0, len0)

	ptr1, len1, err := s.passString(ctx, payload.ReqUUID)
	if err != nil {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: pass uuid: %w", err)
	}
	defer s.freeBytes(ctx, ptr1, len1)

	ptr2, len2, err := s.passString(ctx, third)
	if err != nil {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: pass third: %w", err)
	}
	defer s.freeBytes(ctx, ptr2, len2)

	ptr3, len3, err := s.passString(ctx, fourth)
	if err != nil {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: pass fourth: %w", err)
	}
	defer s.freeBytes(ctx, ptr3, len3)

	// Call gs(retptr, ptr0, len0, ptr1, len1, ptr2, len2, ptr3, len3).
	if _, err = s.gs.Call(ctx,
		uint64(retPtr),
		uint64(ptr0), uint64(len0),
		uint64(ptr1), uint64(len1),
		uint64(ptr2), uint64(len2),
		uint64(ptr3), uint64(len3),
	); err != nil {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: gs call: %w", err)
	}

	// Read result from stack pointer: resultPtr at retPtr, resultLen at retPtr+4.
	mem := s.memory
	resultPtr, ok := mem.ReadUint32Le(retPtr)
	if !ok {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: read result pointer out of bounds")
	}
	resultLen, ok := mem.ReadUint32Le(retPtr + 4)
	if !ok {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: read result length out of bounds")
	}

	if resultPtr == 0 || resultLen == 0 {
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: gs returned empty result (ptr=%d len=%d)", resultPtr, resultLen)
	}

	resultBytes, ok := mem.Read(resultPtr, resultLen)
	if !ok {
		s.freeBytes(ctx, resultPtr, resultLen)
		s.restoreStackPtr(ctx)
		return nil, fmt.Errorf("signing: read signature bytes out of bounds")
	}
	signature := string(resultBytes)

	// Free result string memory and restore stack.
	s.freeBytes(ctx, resultPtr, resultLen)
	s.restoreStackPtr(ctx)

	return &SigningResult{
		Signature: signature,
		Timestamp: payload.Timestamp,
		ReqUUID:   payload.ReqUUID,
	}, nil
}

func (s *wazeroSigner) restoreStackPtr(ctx context.Context) {
	_, _ = s.stackPtr.Call(ctx, api.EncodeI32(16))
}

// Health reports whether the WASM module is still loaded and operational.
func (s *wazeroSigner) Health() error {
	if s.mod == nil || s.mod.IsClosed() {
		return fmt.Errorf("signer: WASM module is closed")
	}
	if s.gs == nil {
		return fmt.Errorf("signer: gs export missing")
	}
	return nil
}

// Close releases the WASM runtime and all associated resources.
func (s *wazeroSigner) Close(ctx context.Context) error {
	if s.mod != nil {
		s.mod.Close(ctx)
	}
	if s.compiled != nil {
		s.compiled.Close(ctx)
	}
	if s.runtime != nil {
		s.runtime.Close(ctx)
	}
	return nil
}

// passString encodes a Go string into WASM memory and returns pointer and length.
func (s *wazeroSigner) passString(ctx context.Context, value string) (ptr uint32, length uint32, err error) {
	encoded := []byte(value)
	blen := uint32(len(encoded))
	results, err := s.wMalloc.Call(ctx, uint64(blen), 1)
	if err != nil {
		return 0, 0, err
	}
	ptr = uint32(results[0])
	s.memory.Write(ptr, encoded)
	return ptr, blen, nil
}

// freeBytes releases WASM memory allocated for a string.
func (s *wazeroSigner) freeBytes(ctx context.Context, ptr, length uint32) {
	if ptr != 0 {
		_, _ = s.wFree.Call(ctx, uint64(ptr), uint64(length), 1)
	}
}
