// A plugin whose renewal leaves the sign-in due: its refreshLead is an
// hour, and a renewal it can't make keeps the token it has and pushes
// expires out by half an hour only, which is still inside that hour. The
// refresh token says what the renewal gives back: "d-keep…" only the
// short expires, "d-new…" a new token with the same short expires, and
// "d-none…" nothing. Each renewal is written to $RENEW_LOG as a line
// naming the refresh token it was given. Its models hook is what has a
// listing renew a due sign-in first, as a plugin that asks its vendor.
import fs from "node:fs"

const LEAD = 60 * 60 * 1000

export const RenewDuePlugin = async () => ({
  config: async (cfg) => {
    cfg.provider = cfg.provider ?? {}
    cfg.provider.dueco = {
      name: "DueCo",
      npm: "@ai-sdk/openai-compatible",
      api: "https://due.invalid/v1",
      models: { "due-1": { name: "Due One", limit: { context: 1000, output: 100 } } },
    }
  },
  provider: {
    id: "dueco",
    models: async (provider) => provider.models,
  },
  auth: {
    provider: "dueco",
    refreshLead: LEAD,
    methods: [{ type: "api", label: "API key" }],
    async refresh(auth) {
      fs.appendFileSync(process.env.RENEW_LOG, auth.refresh + "\n")
      const expires = Date.now() + 30 * 60 * 1000
      if (auth.refresh.startsWith("d-none")) return undefined
      if (auth.refresh.startsWith("d-new")) return { access: auth.access + "+", refresh: auth.refresh + "+", expires }
      return { expires }
    },
  },
})
