#!/usr/bin/env node

import { createInterface } from "node:readline";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";

const protocolWrite = process.stdout.write.bind(process.stdout);
process.stdout.write = process.stderr.write.bind(process.stderr);

function argValue(name) {
  const index = process.argv.indexOf(name);
  if (index < 0 || index + 1 >= process.argv.length) return "";
  return process.argv[index + 1] ?? "";
}

function resolveGpuMode(value = process.env.AFS_LLAMA_GPU) {
  const normalized = (value ?? "").trim().toLowerCase();
  if (!normalized) return "auto";
  if (["false", "off", "none", "disable", "disabled", "0"].includes(normalized)) return false;
  if (["metal", "vulkan", "cuda", "auto"].includes(normalized)) return normalized;
  process.stderr.write(`AFS Warning: invalid AFS_LLAMA_GPU="${value}", using auto GPU selection.\n`);
  return "auto";
}

async function loadNodeLlamaCpp() {
  const require = createRequire(import.meta.url);
  const moduleName = (process.env.AFS_NODE_LLAMA_CPP_MODULE ?? "node-llama-cpp").trim();
  const attempts = [moduleName];
  try {
    const globalRoot = execFileSync("npm", ["root", "-g"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
    if (globalRoot) {
      attempts.push(`${globalRoot}/node-llama-cpp`);
      attempts.push(`${globalRoot}/@tobilu/qmd/node_modules/node-llama-cpp`);
    }
  } catch {
    // npm is optional; normal module resolution may still work.
  }
  const errors = [];
  for (const candidate of attempts) {
    try {
      return await import(candidate);
    } catch (importError) {
      try {
        const resolved = require.resolve(candidate);
        return await import(pathToFileURL(resolved).href);
      } catch (resolveError) {
        errors.push(`${candidate}: ${resolveError instanceof Error ? resolveError.message : String(importError)}`);
      }
    }
  }
  throw new Error(
    `node-llama-cpp is unavailable. Install the AFS local embedding helper dependency or set AFS_NODE_LLAMA_CPP_MODULE. ${errors.join("; ")}`
  );
}

const modelUri = argValue("--model");
const cacheDir = argValue("--cache-dir");
const dimensions = Number.parseInt(argValue("--dimensions") || "0", 10);

if (!modelUri) {
  throw new Error("--model is required");
}
if (!cacheDir) {
  throw new Error("--cache-dir is required");
}

const { getLlama, resolveModelFile, LlamaLogLevel } = await loadNodeLlamaCpp();

async function createLlama() {
  const gpu = resolveGpuMode();
  const load = async (gpuMode) =>
    await getLlama({
      build: "autoAttempt",
      logLevel: LlamaLogLevel.error,
      gpu: gpuMode,
    });
  if (gpu === false) return await load(false);
  try {
    return await load(gpu);
  } catch (error) {
    process.stderr.write(
      `AFS Warning: GPU init failed${gpu === "auto" ? "" : ` for AFS_LLAMA_GPU=${gpu}`} (${error instanceof Error ? error.message : String(error)}), falling back to CPU.\n`
    );
    return await load(false);
  }
}

const llama = await createLlama();
const modelPath = await resolveModelFile(modelUri, cacheDir);
const model = await llama.loadModel({ modelPath });
const embedContextSize = Number.parseInt(process.env.AFS_EMBED_CONTEXT_SIZE ?? "2048", 10);
const context = await model.createEmbeddingContext({
  contextSize: embedContextSize,
});
process.stdout.write = protocolWrite;

function embedTokenLimit() {
  const trained = model.trainContextSize;
  if (Number.isFinite(trained) && trained > 0) {
    return Math.max(1, Math.min(embedContextSize, trained));
  }
  return embedContextSize;
}

function truncateForEmbedding(text) {
  const normalized = text || " ";
  const limit = embedTokenLimit();
  const tokens = model.tokenize(normalized);
  if (tokens.length <= limit) {
    return normalized;
  }
  const safeLimit = Math.max(1, limit - 4);
  const truncated = model.detokenize(tokens.slice(0, safeLimit));
  process.stderr.write(`AFS Warning: truncated embedding input from ${tokens.length} to ${safeLimit} tokens.\n`);
  return truncated || " ";
}

async function embedBatch(texts) {
  const vectors = [];
  for (const text of texts) {
    const embedding = await context.getEmbeddingFor(truncateForEmbedding(text));
    const vector = Array.from(embedding.vector);
    if (dimensions > 0 && vector.length !== dimensions) {
      throw new Error(`local embedding dimension = ${vector.length}, want ${dimensions}`);
    }
    vectors.push(vector);
  }
  return vectors;
}

const rl = createInterface({
  input: process.stdin,
  crlfDelay: Infinity,
});

for await (const line of rl) {
  if (!line.trim()) continue;
  let request;
  try {
    request = JSON.parse(line);
    if (request.op === "ready") {
      protocolWrite(JSON.stringify({ id: request.id, path: modelPath }) + "\n");
      continue;
    }
    if (request.op !== "embedBatch") {
      throw new Error(`unknown op ${request.op}`);
    }
    const vectors = await embedBatch(Array.isArray(request.texts) ? request.texts : []);
    protocolWrite(JSON.stringify({ id: request.id, vectors }) + "\n");
  } catch (error) {
    const id = request?.id ?? null;
    protocolWrite(
      JSON.stringify({ id, error: error instanceof Error ? error.message : String(error) }) + "\n"
    );
  }
}

await context.dispose();
await model.dispose();
