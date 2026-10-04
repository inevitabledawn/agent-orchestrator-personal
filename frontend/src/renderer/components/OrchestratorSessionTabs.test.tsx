import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import type { WorkspaceSession } from "../types/workspace";
import { OrchestratorSessionTabs } from "./OrchestratorSessionTabs";

const { navigate, spawn, terminate } = vi.hoisted(() => ({ navigate: vi.fn(), spawn: vi.fn(), terminate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => navigate }));
vi.mock("../lib/spawn-orchestrator", () => ({ spawnOrchestrator: spawn }));
vi.mock("../hooks/useTerminateSession", () => ({ useTerminateSession: () => ({ mutateAsync: terminate }) }));

const sessions = [
	{ id: "older", workspaceId: "project", kind: "orchestrator", mode: "chat", status: "working", isTerminated: false, createdAt: "2026-09-27T00:00:00Z" },
	{ id: "current", workspaceId: "project", kind: "orchestrator", mode: "chat", status: "working", isTerminated: false, createdAt: "2026-09-28T00:00:00Z" },
] as WorkspaceSession[];

beforeEach(() => {
	navigate.mockReset();
	spawn.mockReset();
	terminate.mockReset().mockResolvedValue(undefined);
	window.localStorage.clear();
});

it("opens an older orchestrator session without spawning or resuming it", async () => {
	const queryClient = new QueryClient();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("tab", { name: /Session 1/ }));
	expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "older" } });
	expect(spawn).not.toHaveBeenCalled();
});

it("creates a fresh orchestrator using the daemon's default interface, then opens it", async () => {
	spawn.mockResolvedValue("new-session");
	const queryClient = new QueryClient();
	const invalidate = vi.spyOn(queryClient, "invalidateQueries");
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("button", { name: "New orchestrator session" }));
	await waitFor(() => expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "new-session" } }));
	expect(spawn).toHaveBeenCalledExactlyOnceWith("project", "session_tab", false, undefined, undefined, true);
	expect(invalidate).toHaveBeenCalledWith({ queryKey: ["workspaces"] });
});

it("shows the current session as a selectable tab without a separate Orchestrator tab", () => {
	const queryClient = new QueryClient();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} tabAction={<button type="button">Session actions</button>} /></QueryClientProvider>);
	expect(screen.getAllByRole("tab")).toHaveLength(2);
	expect(screen.getByRole("tab", { name: "Session 2" })).toHaveAttribute("aria-selected", "true");
	expect(screen.getByRole("button", { name: "Session actions" })).toBeInTheDocument();
});

it("closing the last tab terminates it and opens the orchestration menu", async () => {
	const queryClient = new QueryClient();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={[sessions[1]]} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("button", { name: "Close Session 1" }));
	expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/orchestrators", params: { projectId: "project" } });
	expect(spawn).not.toHaveBeenCalled();
	expect(terminate).toHaveBeenCalledWith(sessions[1]);
});

it("terminates an inactive tab and keeps it closed after remount", async () => {
	const queryClient = new QueryClient();
	const view = render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("button", { name: "Close Session 1" }));
	expect(screen.queryByRole("tab", { name: /Session 1/ })).not.toBeInTheDocument();
	expect(navigate).not.toHaveBeenCalled();
	view.unmount();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	expect(screen.queryByRole("tab", { name: /Session 1/ })).not.toBeInTheDocument();
});

it("does not restore terminated sessions as tabs", () => {
 const queryClient = new QueryClient();
 render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={[{ ...sessions[0], isTerminated: true }, sessions[1]]} /></QueryClientProvider>);
 expect(screen.queryByRole("tab", { name: /Session 1/ })).not.toBeInTheDocument();
});

it("keeps the tab open when termination fails", async () => {
 terminate.mockRejectedValue(new Error("Cannot stop session"));
 const queryClient = new QueryClient();
 render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
 await userEvent.click(screen.getByRole("button", { name: "Close Session 2" }));
 expect(await screen.findByRole("alert")).toHaveTextContent("Cannot stop session");
 expect(screen.getByRole("tab", { name: "Session 2" })).toBeInTheDocument();
 expect(navigate).not.toHaveBeenCalled();
});

it("keeps the current session open and shows a spawn error", async () => {
	spawn.mockRejectedValue(new Error("Cannot create parallel session"));
	const queryClient = new QueryClient();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("button", { name: "New orchestrator session" }));
	expect(await screen.findByRole("alert")).toHaveTextContent("Cannot create parallel session");
	expect(navigate).not.toHaveBeenCalled();
});

it("does not send two parallel spawn requests for rapid clicks", async () => {
	spawn.mockReturnValue(new Promise(() => undefined));
	const queryClient = new QueryClient();
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	const button = screen.getByRole("button", { name: "New orchestrator session" });
	fireEvent.click(button);
	fireEvent.click(button);
	expect(spawn).toHaveBeenCalledTimes(1);
});

it("opens the created session even when refreshing the workspace list fails", async () => {
	spawn.mockResolvedValue("new-session");
	const queryClient = new QueryClient();
	vi.spyOn(queryClient, "invalidateQueries").mockRejectedValue(new Error("Refresh failed"));
	render(<QueryClientProvider client={queryClient}><OrchestratorSessionTabs currentSessionId="current" projectId="project" sessions={sessions} /></QueryClientProvider>);
	await userEvent.click(screen.getByRole("button", { name: "New orchestrator session" }));
	await waitFor(() => expect(navigate).toHaveBeenCalledWith({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId: "project", sessionId: "new-session" } }));
});

it("remembers the selected session across leaving and remounting without closing any tabs", () => {
 const client = new QueryClient();
 const view = render(<QueryClientProvider client={client}><OrchestratorSessionTabs currentSessionId="older" projectId="project" sessions={sessions} /></QueryClientProvider>);
 view.unmount();
 expect(localStorage.getItem("ao.orchestrator.lastSession.project")).toBe("older");
 render(<QueryClientProvider client={client}><OrchestratorSessionTabs currentSessionId="older" projectId="project" sessions={sessions} /></QueryClientProvider>);
 expect(screen.getAllByRole("tab")).toHaveLength(2);
 expect(terminate).not.toHaveBeenCalled();
});
