/** Navigation preferences never change the daemon's session lifecycle. */
export function rememberOrchestratorSession(projectId: string, sessionId: string): void {
	try {
		window.localStorage.setItem(`ao.orchestrator.lastSession.${projectId}`, sessionId);
	} catch { /* Navigation still works without persistent storage. */ }
}

export function readLastOrchestratorSession(projectId: string): string | null {
	try {
		return window.localStorage.getItem(`ao.orchestrator.lastSession.${projectId}`);
	} catch {
		return null;
	}
}

/** Legacy hidden tabs stay closed when upgrading to durable session closure. */
export function readClosedOrchestratorTabs(projectId: string): string[] {
	try {
		const stored: unknown = JSON.parse(window.localStorage.getItem(`ao.orchestrator.closedTabs.${projectId}`) ?? "[]");
		return Array.isArray(stored) ? stored.filter((id): id is string => typeof id === "string") : [];
	} catch {
		return [];
	}
}
