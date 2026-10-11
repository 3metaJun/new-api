package billing_setting

// Built-in token prices use actual USD per million tokens. Keep new model
// defaults here instead of splitting them across the legacy ratio tables.
// Ported from main (upstream eb99ab1b4 and 3metaJun/new-api#15).
var builtinBillingExpr = map[string]string{
	// https://developers.openai.com/api/docs/models/gpt-6-luna
	"gpt-6-luna": `tier("base", p * 0.1 + c * 0.5 + cr * 0.01 + cc * 0.125)`,
	// https://platform.claude.com/docs/en/about-claude/pricing
	// Prompts above 100K tokens bill every category at five times the standard
	// rate. Cache writes follow Anthropic's 1.25x (5 minutes) and 2x (1 hour).
	"claude-haiku-5-5": `len <= 100000 ? tier("standard", p * 0.1 + c * 0.5 + cr * 0.01 + cc * 0.125 + cc1h * 0.2) : tier("long_context", p * 0.5 + c * 2.5 + cr * 0.05 + cc * 0.625 + cc1h * 1)`,
	// https://ai.google.dev/gemini-api/docs/pricing#gemini-3.8-flash
	// Introductory price through 2026-12-31; Google lists $1.50 / $7.50 from
	// 2027-01-01, which needs an admin override or a new default by then.
	"gemini-3.8-flash": `tier("base", p * 0.75 + c * 3.75 + cr * 0.075)`,
}
