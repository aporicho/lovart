#!/usr/bin/env node

const fs = require("node:fs/promises");
const path = require("node:path");

const ROOT = path.resolve(__dirname, "..", "..");
const DEFAULT_WASM = path.join(
  ROOT,
  "lovart_reverse",
  "data",
  "ref",
  "lovart_static_assets",
  "26bd3a5bd74c3c92.wasm",
);

let wasm;
let cachedDataViewMemory0 = null;
let cachedUint8ArrayMemory0 = null;
let wasmVectorLen = 0;

let cachedTextDecoder = new TextDecoder("utf-8", {
  ignoreBOM: true,
  fatal: true,
});
cachedTextDecoder.decode();

const cachedTextEncoder = new TextEncoder();

function getDataViewMemory0() {
  if (
    cachedDataViewMemory0 === null ||
    cachedDataViewMemory0.buffer.detached === true ||
    (cachedDataViewMemory0.buffer.detached === undefined &&
      cachedDataViewMemory0.buffer !== wasm.memory.buffer)
  ) {
    cachedDataViewMemory0 = new DataView(wasm.memory.buffer);
  }
  return cachedDataViewMemory0;
}

function getUint8ArrayMemory0() {
  if (cachedUint8ArrayMemory0 === null || cachedUint8ArrayMemory0.byteLength === 0) {
    cachedUint8ArrayMemory0 = new Uint8Array(wasm.memory.buffer);
  }
  return cachedUint8ArrayMemory0;
}

function passStringToWasm0(value, malloc, realloc) {
  if (realloc === undefined) {
    const buffer = cachedTextEncoder.encode(value);
    const ptr = malloc(buffer.length, 1) >>> 0;
    getUint8ArrayMemory0()
      .subarray(ptr, ptr + buffer.length)
      .set(buffer);
    wasmVectorLen = buffer.length;
    return ptr;
  }

  let len = value.length;
  let ptr = malloc(len, 1) >>> 0;
  const memory = getUint8ArrayMemory0();

  let offset = 0;
  for (; offset < len; offset += 1) {
    const code = value.charCodeAt(offset);
    if (code > 0x7f) {
      break;
    }
    memory[ptr + offset] = code;
  }

  if (offset !== len) {
    if (offset !== 0) {
      value = value.slice(offset);
    }
    ptr = realloc(ptr, len, (len = offset + value.length * 3), 1) >>> 0;
    const view = getUint8ArrayMemory0().subarray(ptr + offset, ptr + len);
    const result = cachedTextEncoder.encodeInto(value, view);
    offset += result.written;
    ptr = realloc(ptr, len, offset, 1) >>> 0;
  }

  wasmVectorLen = offset;
  return ptr;
}

function getStringFromWasm0(ptr, len) {
  ptr >>>= 0;
  return cachedTextDecoder.decode(getUint8ArrayMemory0().subarray(ptr, ptr + len));
}

async function init(wasmPath = DEFAULT_WASM) {
  if (wasm) {
    return wasm;
  }
  const bytes = await fs.readFile(wasmPath);
  const imports = { wbg: {} };
  const { instance, module } = await WebAssembly.instantiate(bytes, imports);
  wasm = instance.exports;
  init.__wbindgen_wasm_module = module;
  cachedDataViewMemory0 = null;
  cachedUint8ArrayMemory0 = null;
  return wasm;
}

function sign(timestamp, reqUuid, third = "", fourth = "") {
  if (!wasm) {
    throw new Error("WASM not initialized; call init() first");
  }

  let retptr = 0;
  let resultPtr = 0;
  let resultLen = 0;

  try {
    retptr = wasm.__wbindgen_add_to_stack_pointer(-16);
    const ptr0 = passStringToWasm0(String(timestamp), wasm.__wbindgen_export2, wasm.__wbindgen_export3);
    const len0 = wasmVectorLen;
    const ptr1 = passStringToWasm0(String(reqUuid), wasm.__wbindgen_export2, wasm.__wbindgen_export3);
    const len1 = wasmVectorLen;
    const ptr2 = passStringToWasm0(String(third), wasm.__wbindgen_export2, wasm.__wbindgen_export3);
    const len2 = wasmVectorLen;
    const ptr3 = passStringToWasm0(String(fourth), wasm.__wbindgen_export2, wasm.__wbindgen_export3);
    const len3 = wasmVectorLen;

    wasm.gs(retptr, ptr0, len0, ptr1, len1, ptr2, len2, ptr3, len3);
    resultPtr = getDataViewMemory0().getInt32(retptr + 0, true);
    resultLen = getDataViewMemory0().getInt32(retptr + 4, true);
    return getStringFromWasm0(resultPtr, resultLen);
  } finally {
    if (retptr) {
      wasm.__wbindgen_add_to_stack_pointer(16);
    }
    if (resultPtr) {
      wasm.__wbindgen_export(resultPtr, resultLen, 1);
    }
  }
}

function verify(token) {
  if (!wasm) {
    throw new Error("WASM not initialized; call init() first");
  }
  const ptr0 = passStringToWasm0(String(token), wasm.__wbindgen_export2, wasm.__wbindgen_export3);
  const len0 = wasmVectorLen;
  return wasm.ve(ptr0, len0) !== 0;
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length < 2 || args.includes("--help") || args.includes("-h")) {
    console.error("Usage: node lovart_reverse/signing/signature.js <timestamp> <req_uuid> [third] [fourth] [wasm_path]");
    process.exit(args.length < 2 ? 2 : 0);
  }

  const [timestamp, reqUuid, third = "", fourth = "", wasmPath = DEFAULT_WASM] = args;
  await init(wasmPath);
  process.stdout.write(sign(timestamp, reqUuid, third, fourth) + "\n");
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error && error.stack ? error.stack : String(error));
    process.exit(1);
  });
}

module.exports = { init, sign, verify, DEFAULT_WASM };
