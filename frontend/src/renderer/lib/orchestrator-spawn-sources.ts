export const ORCHESTRATOR_SPAWN_SOURCES = [
	"board",
	"restore_dialog",
	"topbar",
	"sidebar",
	"project_add",
	"project_clone",
	"settings",
	"restart",
	"session_tab",
	"command_palette",
] as const;

export type OrchestratorSpawnSource = (typeof ORCHESTRATOR_SPAWN_SOURCES)[number];
