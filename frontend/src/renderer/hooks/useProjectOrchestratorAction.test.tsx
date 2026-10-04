import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useProjectOrchestratorAction } from "./useProjectOrchestratorAction";
import type { WorkspaceSession } from "../types/workspace";

const { navigate, spawn, resume } = vi.hoisted(() => ({ navigate: vi.fn(), spawn: vi.fn(), resume: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));
vi.mock("../lib/spawn-orchestrator", () => ({ spawnOrchestrator: spawn, resumeOrchestrator: resume, isChatPreflightError: () => false }));
beforeEach(() => { navigate.mockReset(); spawn.mockReset(); resume.mockReset(); });
function setup(orchestrator?: WorkspaceSession) {
	const client = new QueryClient();
	return renderHook(() => useProjectOrchestratorAction({ projectId: "project", project: { id: "project", name: "Project", orchestratorAgent: "codex" }, orchestrator, source: "board" }), {
		wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
	});
}
it.each([undefined, { id: "old", mode: "tui", kind: "orchestrator", activity: { state: "exited" } } as WorkspaceSession])("opens the menu without spawning or resuming with session %s", (session) => {
	const { result } = setup(session);
	act(() => result.current.openOrchestrator());
	expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/orchestrators", params: { projectId: "project" } });
	expect(spawn).not.toHaveBeenCalled();
	expect(resume).not.toHaveBeenCalled();
});
it("creates a new independent session instead of reusing the previous session", async () => {
	spawn.mockResolvedValue("fresh");
	const { result } = setup({ id: "old", kind: "orchestrator" } as WorkspaceSession);
	act(() => result.current.createOrchestrator());
	await waitFor(() => expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "fresh" } }));
	expect(spawn).toHaveBeenCalledWith("project", "board", false, undefined, undefined, true);
	expect(resume).not.toHaveBeenCalled();
});
