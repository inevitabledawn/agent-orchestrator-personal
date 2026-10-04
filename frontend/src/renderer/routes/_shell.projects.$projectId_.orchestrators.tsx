import { createFileRoute } from "@tanstack/react-router";
import { SessionsBoard } from "../components/SessionsBoard";

export const Route = createFileRoute("/_shell/projects/$projectId_/orchestrators")({
	component: ProjectOrchestratorsRoute,
});

function ProjectOrchestratorsRoute() {
	const { projectId } = Route.useParams();
	return <SessionsBoard projectId={projectId} orchestrationOnly />;
}
