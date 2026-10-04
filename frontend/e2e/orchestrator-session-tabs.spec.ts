import { expect, test } from "@playwright/test";
import { installFakeAgent, type FakeAgentController } from "./support/fake-bridge";

const projectId = "orchestrator-tabs";
const first = `${projectId}-orchestrator`;
const second = `${projectId}-second`;

test("orchestrator sessions survive navigation and explicit closure creates fresh sessions @T0", async ({ page }, testInfo) => {
	await page.emulateMedia({ reducedMotion: "reduce" });
	await installFakeAgent(page, { projectId, projectName: "Orchestrator Tabs", parallelOrchestrators: true, platform: "MacIntel", orchestrators: [{ id: second, title: "Second orchestrator", mode: "tui" }] });
	let created = 0;
	let resumed = 0;
	const killed: string[] = [];
	await page.route("http://127.0.0.1:8080/api/v1/**", async (route) => {
		const pathname = new URL(route.request().url()).pathname;
		if (pathname.endsWith("/resume-agent")) resumed++;
		if (pathname.endsWith("/kill")) {
			const id = pathname.split("/")[4];
			killed.push(id);
			await page.evaluate((id) => (window as unknown as { __aoFakeAgent: FakeAgentController }).__aoFakeAgent.removeWorker(id), id);
		}
		if (pathname === "/api/v1/orchestrators" && route.request().method() === "POST") {
			expect(route.request().postDataJSON()).toMatchObject({ projectId, parallel: true });
			const id = `${projectId}-fresh-${++created}`;
			await page.evaluate((id) => (window as unknown as { __aoFakeAgent: FakeAgentController }).__aoFakeAgent.createOrchestrator({ id, title: "Fresh orchestrator", mode: "tui" }), id);
			await route.fulfill({ json: { orchestrator: { id } } });
			return;
		}
		if (pathname === "/api/v1/agents/readiness" || pathname === "/api/v1/agents/readiness/ensure") {
			await route.fulfill({ json: { agents: [] } }); return;
		}
		if (pathname === `/api/v1/projects/${projectId}`) {
			await route.fulfill({ json: { status: "ok", project: { id: projectId, agent: "codex", config: { orchestrator: { agent: "codex" } } } } }); return;
		}
		await route.fulfill({ json: { status: "ok" } });
	});
	await page.goto(`/#/projects/${projectId}/sessions/${first}`);
	await expect(page.getByRole("tab", { name: "Session 1" })).toHaveAttribute("aria-selected", "true");
	await expect(page.getByRole("tab", { name: "Session 2" })).toBeVisible();
	const plus = page.getByRole("button", { name: "New orchestrator session", exact: true });
	const buttonBox = await plus.boundingBox();
	const iconBox = await plus.locator("svg").boundingBox();
	const tabBox = await page.getByRole("tab", { name: "Session 1" }).boundingBox();
	expect(buttonBox).not.toBeNull(); expect(iconBox).not.toBeNull(); expect(tabBox).not.toBeNull();
	expect(Math.abs(iconBox!.y + iconBox!.height / 2 - tabBox!.y - tabBox!.height / 2)).toBeLessThan(1);
	await page.screenshot({ path: testInfo.outputPath("centered-orchestrator-tabs.png") });
	await page.getByRole("tab", { name: "Session 2" }).click();
	await expect(page).toHaveURL(new RegExp(`/sessions/${second}$`));
	// Leaving orchestration must neither terminate sessions nor forget selection.
	for (const selected of [second, first, second]) {
		await page.getByRole("tab", { name: selected === first ? "Session 1" : "Session 2" }).click();
		await expect(page).toHaveURL(new RegExp(`/sessions/${selected}$`));
		await page.getByRole("button", { name: "Open Kanban", exact: true }).click();
		await expect(page).toHaveURL(new RegExp(`/projects/${projectId}$`));
		await expect(page.getByText("No orchestrator is running for this project.")).toHaveCount(0);
		await expect(page.getByText("Multiple live orchestrator sessions detected")).toHaveCount(0);
		await page.getByRole("button", { name: "Open Orchestrator Tabs orchestrator" }).click();
		await expect(page).toHaveURL(new RegExp(`/sessions/${selected}$`));
		await expect(page.getByRole("tab", { name: "Session 1" })).toBeVisible();
		await expect(page.getByRole("tab", { name: "Session 2" })).toBeVisible();
		await expect(page.getByTestId("orchestrator-menu")).toHaveCount(0);
		expect(killed).toEqual([]);
		expect(created).toBe(0); expect(resumed).toBe(0);
	}
	await page.goto(`/#/projects/${projectId}/orchestrators`);
	await expect(page).toHaveURL(new RegExp(`/sessions/${second}$`));
	await page.reload();
	await expect(page.getByRole("tab", { name: "Session 2" })).toHaveAttribute("aria-selected", "true");
	await page.screenshot({ path: testInfo.outputPath("restored-orchestrator-session.png") });
	await page.getByRole("button", { name: "Close Session 2" }).click();
	await expect(page).toHaveURL(new RegExp(`/sessions/${first}$`));
	await expect(page.getByRole("tab", { name: "Session 2" })).toHaveCount(0);
	await page.getByRole("button", { name: "Close Session 1" }).click();
	await expect(page).toHaveURL(new RegExp(`/projects/${projectId}/orchestrators$`));
	await expect(page.getByTestId("project-board-empty")).toBeVisible();
	expect(killed).toEqual([second, first]);
	expect(created).toBe(0); expect(resumed).toBe(0);
	await page.getByRole("button", { name: "Open Orchestrator Tabs orchestrator" }).click();
	await expect(page.getByTestId("project-board-empty")).toBeVisible();
	expect(created).toBe(0); expect(resumed).toBe(0);
	await page.reload();
	await expect(page.getByTestId("project-board-empty")).toBeVisible();
	await page.getByTestId("project-board-empty").getByRole("button", { name: "Spawn Orchestrator" }).click();
	await expect(page).toHaveURL(new RegExp(`/sessions/${projectId}-fresh-1$`));
	expect(created).toBe(1); expect(resumed).toBe(0);
	await page.getByRole("button", { name: "New orchestrator session", exact: true }).click();
	await expect(page).toHaveURL(new RegExp(`/sessions/${projectId}-fresh-2$`));
	expect(created).toBe(2);
});
