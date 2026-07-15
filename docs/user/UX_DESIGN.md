# Console design

The normative console and machine-output contract is [CLI_SPEC.md](CLI_SPEC.md). Preserve the current box-drawn, information-dense visual language and semantic status colors. Below 60 columns use stacked cards; 60–99 use compact tables/cards; 100+ may use full layouts. Limited terminals degrade to plain ASCII.

Dashboards refresh inline, never enter the alternate screen, and have one owner for clear/resize/cursor/cancellation lifecycle. Feature renderers consume view models and use `internal/console`; they do not introduce new terminal-control or palette frameworks.
