import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import type { ProjectOrchestratorAction } from "../hooks/useProjectOrchestratorAction";
import { readClosedOrchestratorTabs, readLastOrchestratorSession } from "../lib/orchestrator-tabs";
import { isOrchestratorSession, type WorkspaceSession } from "../types/workspace";
import { ProjectBoardEmpty } from "./BoardEmptyStates";
import { ProjectBoardActions } from "./ProjectBoardActions";

export function OrchestratorMenu({ projectId, sessions, actions }: {
	projectId: string;
	sessions: WorkspaceSession[];
	actions: ProjectOrchestratorAction;
}) {
	const navigate = useNavigate();
	const closed = readClosedOrchestratorTabs(projectId);
	const orchestrators = sessions.filter(isOrchestratorSession)
		.sort((a, b) => (a.createdAt ?? "").localeCompare(b.createdAt ?? "") || a.id.localeCompare(b.id));
	const visible = orchestrators.filter((session) => !session.isTerminated && !closed.includes(session.id));
	const lastSessionId = readLastOrchestratorSession(projectId);
	const sessionId = (visible.find((session) => session.id === lastSessionId) ?? visible.at(-1))?.id;
	useEffect(() => {
		if (sessionId) {
			void navigate({
				to: "/projects/$projectId/sessions/$sessionId",
				params: { projectId, sessionId },
				replace: true,
			});
		}
	}, [navigate, projectId, sessionId]);
	if (visible.length === 0) return <ProjectBoardEmpty actions={<ProjectBoardActions actions={actions} placement="empty" />} />;
	// Return directly to the conversation without flashing a session chooser.
	return null;
}
