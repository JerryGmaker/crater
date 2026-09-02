#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const expectedSwaggerPaths = [
  "/v1/jobtemplate",
  "/v1/vcjobs",
  "/v1/vcjobs/all",
  "/v1/vcjobs/user/{username}",
  "/v1/vcjobs/facets",
  "/v1/vcjobs/all/facets",
  "/v1/vcjobs/user/{username}/facets",
  "/v1/approvalorder/page",
  "/v1/admin/approvalorder/page",
  "/v1/admin/gpu-analysis/page",
  "/v1/aijobs/page",
  "/v1/aijobs/page/facets",
  "/v1/aijobs/all/page",
  "/v1/aijobs/all/page/facets",
  "/v1/aijobs/user/{username}/page",
  "/v1/aijobs/user/{username}/page/facets",
  "/v1/admin/aijobs/page",
  "/v1/admin/aijobs/page/facets",
  "/v1/admin/aijobs/user/{username}/page",
  "/v1/admin/aijobs/user/{username}/page/facets",
];

const frontendGuards = [
  {
    name: "JobTemplate",
    file: "frontend/src/routes/portal/templates/index.tsx",
    markers: ["useQuery", "listJobTemplate", "pageSize", "remote={{"],
  },
  {
    name: "VCJob",
    file: "frontend/src/routes/portal/overview/index.tsx",
    markers: ["RemoteDataTable", "buildRemoteQueryKey", "apiJobAllList"],
  },
  {
    name: "ApprovalOrder",
    file: "frontend/src/routes/portal/more/orders/index.tsx",
    markers: ["useRemoteTableState", "apiGetApprovalOrderPage", "remoteState"],
  },
  {
    name: "GPU Analysis",
    file: "frontend/src/routes/admin/gpu-analysis/-components/gpu-analysis-table.tsx",
    markers: [
      "RemoteDataTable",
      "apiAdminListGpuAnalysesPaged",
      "buildRemoteQueryKey",
    ],
  },
  {
    name: "EMIAS",
    file: "frontend/src/components/job/overview/emias-jobs.tsx",
    markers: ["RemoteDataTable", "apiJobBatchList", "buildRemoteQueryKey"],
  },
];

async function readText(relativePath) {
  return readFile(resolve(ROOT, relativePath), "utf8");
}

export async function runStaticChecks({ log = console.log } = {}) {
  const swagger = JSON.parse(await readText("backend/docs/swagger.json"));
  const missingSwaggerPaths = expectedSwaggerPaths.filter(
    (path) => !swagger.paths?.[path],
  );
  if (missingSwaggerPaths.length > 0) {
    throw new Error(
      `Swagger is missing paths: ${missingSwaggerPaths.join(", ")}`,
    );
  }
  log(`Swagger pagination paths: ${expectedSwaggerPaths.length} present`);

  for (const guard of frontendGuards) {
    const source = await readText(guard.file);
    const missingMarkers = guard.markers.filter(
      (marker) => !source.includes(marker),
    );
    if (missingMarkers.length > 0) {
      throw new Error(
        `${guard.name} remote pagination guard failed: ${missingMarkers.join(", ")}`,
      );
    }
    log(`Frontend remote guard: ${guard.name} passed`);
  }

  return {
    swaggerPaths: expectedSwaggerPaths.length,
    frontendGuards: frontendGuards.length,
  };
}

if (
  process.argv[1] &&
  fileURLToPath(import.meta.url) === resolve(process.argv[1])
) {
  runStaticChecks().catch((error) => {
    console.error(`ERROR: ${error.message}`);
    process.exitCode = 1;
  });
}
