import { config } from "../config.js";
import { claudeProvider } from "./claude.js";
import { codexProvider } from "./codex.js";
import { openaiProvider } from "./openai.js";

const providers = new Map([
  [claudeProvider.id, claudeProvider],
  [openaiProvider.id, openaiProvider],
  [codexProvider.id, codexProvider],
]);

export function allProviders() {
  return [...providers.values()];
}

export function getProvider(id) {
  return providers.get(id) ?? null;
}

export function getProviderForTurn(body) {
  const ref = parseModelRef(body.model);
  const providerID = body.provider || ref.provider || config.provider;
  return getProvider(providerID) ?? claudeProvider;
}

export function parseModelRef(model) {
  if (typeof model !== "string") return { provider: "", model: "" };
  const trimmed = model.trim();
  const i = trimmed.indexOf(":");
  if (i <= 0) return { provider: "", model: trimmed };
  return { provider: trimmed.slice(0, i), model: trimmed.slice(i + 1) };
}

export function providerCatalog() {
  return {
    defaultProvider: config.provider,
    defaultModel: modelRef(config.provider, config.model),
    providers: allProviders().map((provider) => ({
      id: provider.id,
      label: provider.label,
      defaultModel: `${provider.id}:${provider.defaultModel}`,
      defaultEffort: provider.defaultEffort,
      capabilities: provider.capabilities,
      models: provider.models.map((model) => ({
        ...model,
        id: `${provider.id}:${model.id}`,
        provider: provider.id,
        model: model.id,
      })),
      efforts: provider.efforts,
    })),
  };
}

function modelRef(provider, model) {
  if (typeof model !== "string" || model.trim() === "") return "";
  const trimmed = model.trim();
  return trimmed.includes(":") ? trimmed : `${provider}:${trimmed}`;
}
