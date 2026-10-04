import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Loader2, Plus, X } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useTerminateSession } from "../hooks/useTerminateSession";
import { useWorkspaceQuery, workspaceQueryKey } from "../hooks/useWorkspaceQuery";
import { readClosedOrchestratorTabs, rememberOrchestratorSession } from "../lib/orchestrator-tabs";
import { spawnOrchestrator } from "../lib/spawn-orchestrator";
import { isOrchestratorSession, type WorkspaceSession } from "../types/workspace";

/** Keep the project-list subscription in the small tab strip, not SessionView. */
export function ProjectOrchestratorTabs({ currentSessionId, projectId, tabAction }: {
	currentSessionId: string;
	projectId: string;
	tabAction?: ReactNode;
}) {
	const workspaces = useWorkspaceQuery().data;
	return (
		<OrchestratorSessionTabs
			key={projectId}
			currentSessionId={currentSessionId}
			projectId={projectId}
			tabAction={tabAction}
			sessions={workspaces?.find((workspace) => workspace.id === projectId)?.sessions ?? []}
		/>
	);
}

export function OrchestratorSessionTabs({ currentSessionId, projectId, sessions, tabAction }: {
	currentSessionId: string;
	projectId: string;
	sessions: WorkspaceSession[];
	tabAction?: ReactNode;
}) {
	const navigate = useNavigate();
	useEffect(() => {
		rememberOrchestratorSession(projectId, currentSessionId);
	}, [projectId, currentSessionId]);
	const queryClient = useQueryClient();
	const { t } = useTranslation();
	const terminate = useTerminateSession();
	const closingIds = useRef(new Set<string>());
	const [closing, setClosing] = useState<string[]>([]);
	const creatingRef = useRef(false);
	const [creating, setCreating] = useState(false);
	const [error, setError] = useState<string>();
	const storageKey = `ao.orchestrator.closedTabs.${projectId}`;
	const [closedIds, setClosedIds] = useState<string[]>(() => readClosedOrchestratorTabs(projectId));
	const updateClosedIds = (ids: string[]) => {
		setClosedIds(ids);
		try { window.localStorage.setItem(storageKey, JSON.stringify(ids)); } catch { /* Tabs still work without storage. */ }
	};
	const orchestrators = sessions
		.filter(isOrchestratorSession)
		.sort((a, b) => (a.createdAt ?? "").localeCompare(b.createdAt ?? "") || a.id.localeCompare(b.id));
	const visible = orchestrators.filter((session) => !session.isTerminated && !closedIds.includes(session.id));
	const open = (sessionId: string) => {
		void navigate({ to: "/projects/$projectId/sessions/$sessionId", params: { projectId, sessionId } });
	};
	const create = async () => {
		if (creatingRef.current) return;
		creatingRef.current = true;
		setCreating(true);
		setError(undefined);
		try {
			const sessionId = await spawnOrchestrator(projectId, "session_tab", false, undefined, undefined, true);
			try {
				await queryClient.invalidateQueries({ queryKey: workspaceQueryKey });
			} catch {
				// The created session has its own detail endpoint. A failed list
				// refresh must not strand the user in the retired conversation.
			}
			open(sessionId);
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : t("orchestratorTabs.createFailed"));
		} finally {
			creatingRef.current = false;
			setCreating(false);
		}
	};
	const close = async (sessionId: string) => {
		const session = visible.find((candidate) => candidate.id === sessionId);
		if (!session || closingIds.current.size > 0) return;
		closingIds.current.add(sessionId);
		setClosing([...closingIds.current]);
		setError(undefined);
		try {
			await terminate.mutateAsync(session);
			updateClosedIds([...closedIds, sessionId]);
			if (sessionId === currentSessionId) {
				const next = visible.filter((candidate) => candidate.id !== sessionId && !closingIds.current.has(candidate.id)).at(-1);
				if (next) open(next.id);
				else void navigate({ to: "/projects/$projectId/orchestrators", params: { projectId } });
			}
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : t("termination.failed", { defaultValue: "Could not close session" }));
		} finally {
			closingIds.current.delete(sessionId);
			setClosing([...closingIds.current]);
		}
	};
	return (
		<>
			{visible.map((session) => {
				const number = orchestrators.indexOf(session) + 1;
				const label = t("orchestratorTabs.session", { number });
				const closeLabel = `${t("common.close")} ${label}`;
				const active = session.id === currentSessionId;
				return <div key={session.id} className={`group relative inline-flex min-w-shell-tab-min max-w-shell-tab-max shrink-0 self-stretch items-center border-r border-border ${active ? "bg-overlay text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-foreground/80" : "text-muted-foreground hover:bg-raised hover:text-foreground"}`}>
					<button
						aria-current={active ? true : undefined}
						aria-label={`${label}${session.isTerminated ? ` · ${t("orchestratorTabs.archived")}` : ""}`}
						aria-selected={active}
						className="min-w-0 flex-1 cursor-pointer truncate py-2 pl-3 pr-1 text-left text-control font-medium focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent/50"
						onClick={() => open(session.id)}
						role="tab"
						tabIndex={active ? 0 : -1}
						title={`${session.title || label}${session.isTerminated ? ` · ${t("orchestratorTabs.archived")}` : ""}`}
						type="button"
					>{label}</button>
					{active && tabAction ? <span className="relative z-10 shrink-0" data-testid="session-tab-action">{tabAction}</span> : null}
					<button aria-label={closeLabel} className={`relative z-10 mr-1 rounded p-1 hover:bg-raised focus:opacity-100 group-hover:opacity-100 ${active ? "opacity-100" : "opacity-0"}`} disabled={closing.length > 0} onClick={() => void close(session.id)} title={closeLabel} type="button"><X aria-hidden="true" className="size-3.5" /></button>
				</div>;
			})}
			<button
				aria-label={t("orchestratorTabs.newSession")}
				className="inline-flex w-10 shrink-0 self-stretch items-center justify-center text-muted-foreground hover:bg-raised hover:text-foreground disabled:opacity-50"
				disabled={creating}
				onClick={() => void create()}
				title={t("orchestratorTabs.newSessionDescription")}
				type="button"
			>
				{creating ? <Loader2 aria-hidden="true" className="size-icon-md animate-spin" /> : <Plus aria-hidden="true" className="size-icon-md" />}
			</button>
			{error ? <span className="self-center px-2 text-xs text-destructive" role="alert">{error}</span> : null}
		</>
	);
}
