import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { TooltipProvider } from "./ui/tooltip";
import { OrchestratorMenu } from "./OrchestratorMenu";
import type { ProjectOrchestratorAction } from "../hooks/useProjectOrchestratorAction";
import type { WorkspaceSession } from "../types/workspace";
const { navigate } = vi.hoisted(() => ({ navigate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));
const create = vi.fn();
const actions = { createOrchestrator: create, openOrchestrator: vi.fn(), openNewTask: vi.fn(), isSpawning: false, isProjectRestarting: false, isProvisioning: false, spawnError: "" } as unknown as ProjectOrchestratorAction;
const old = { id: "old", workspaceId: "project", kind: "orchestrator", createdAt: "2026-09-28", isTerminated: true } as WorkspaceSession;
beforeEach(() => { create.mockReset(); navigate.mockReset(); localStorage.clear(); });
it("shows the empty screen after the last session closes and creates a fresh session", () => {
	render(<TooltipProvider><OrchestratorMenu projectId="project" sessions={[old]} actions={actions} /></TooltipProvider>);
	expect(screen.getByTestId("project-board-empty")).toBeInTheDocument();
	fireEvent.click(screen.getByRole("button", { name: "Spawn Orchestrator" }));
	expect(create).toHaveBeenCalledTimes(1);
	expect(navigate).not.toHaveBeenCalled();
});
it("opens the remaining session automatically and excludes previously closed tabs", () => {
 localStorage.setItem("ao.orchestrator.closedTabs.project", JSON.stringify(["old"]));
 render(<TooltipProvider><OrchestratorMenu projectId="project" sessions={[{ ...old, isTerminated: false }, { ...old, id: "new", createdAt: "2026-09-29", isTerminated: false }]} actions={actions} /></TooltipProvider>);
 expect(screen.queryByTestId("orchestrator-menu")).not.toBeInTheDocument();
 expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "new" }, replace: true });
 expect(create).not.toHaveBeenCalled();
});
it("returns to the last selected session rather than the newest session", () => {
 localStorage.setItem("ao.orchestrator.lastSession.project", "old");
 render(<OrchestratorMenu projectId="project" sessions={[{ ...old, isTerminated: false }, { ...old, id: "new", createdAt: "2026-09-29", isTerminated: false }]} actions={actions} />);
 expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "old" }, replace: true });
 expect(screen.queryByTestId("orchestrator-menu")).not.toBeInTheDocument();
});
it.each(["missing", "old"])("falls back to an open session when the remembered session %s is unavailable", (last) => {
 localStorage.setItem("ao.orchestrator.lastSession.project", last);
 render(<OrchestratorMenu projectId="project" sessions={[old, { ...old, id: "new", isTerminated: false }]} actions={actions} />);
 expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "new" }, replace: true });
});
