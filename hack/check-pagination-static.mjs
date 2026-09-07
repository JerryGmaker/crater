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
  "/v1/admin/accounts/page",
  "/v1/accounts/{aid}/users/page",
  "/v1/admin/accounts/userIn/{aid}/page",
  "/v1/admin/users/page",
  "/v1/admin/gpu-analysis/page",
  "/v1/dataset/mydataset/page",
  "/v1/dataset/{datasetId}/usersIn/page",
  "/v1/dataset/{datasetId}/queuesIn/page",
  "/v1/images/image/page",
  "/v1/admin/images/image/page",
  "/v1/images/kaniko/page",
  "/v1/admin/images/kaniko/page",
  "/v1/model-download/models/downloads/page",
  "/v1/admin/operation-logs/page",
  "/v1/admin/operations/cronjob/record/page",
  "/v1/resources/page",
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
    name: "Admin Accounts",
    file: "frontend/src/routes/admin/accounts/-components/account-table.tsx",
    markers: ["RemoteDataTable", "apiAdminAccountListPaged", "globalSearch"],
  },
  {
    name: "Account Members",
    file: "frontend/src/components/account/account-member-table.tsx",
    markers: [
      "RemoteDataTable",
      "apiUserInProjectListPaged",
      "apiUserListAccountMembersPaged",
      "globalSearch",
    ],
  },
  {
    name: "Admin Users",
    file: "frontend/src/routes/admin/users/index.tsx",
    markers: ["RemoteDataTable", "apiAdminUserListPaged", "globalSearch"],
  },
  {
    name: "Shared Files",
    file: "frontend/src/routes/portal/data/blocks/index.tsx",
    markers: ["apiGetDatasetPaged", 'sourceType="sharefile"'],
  },
  {
    name: "Datasets",
    file: "frontend/src/routes/portal/data/datasets/index.tsx",
    markers: ["apiGetDatasetPaged", 'sourceType="dataset"'],
  },
  {
    name: "Dataset Shares",
    file: "frontend/src/components/file/data-detail.tsx",
    markers: [
      "RemoteDataTable",
      "apiListUsersInDatasetPaged",
      "apiListQueuesInDatasetPaged",
      "globalSearch",
    ],
  },
  {
    name: "Images",
    file: "frontend/src/components/image/images/index.tsx",
    markers: ["RemoteDataTable", "useRemoteTableState", "apiListImage", "globalSearch"],
  },
  {
    name: "Image Builds",
    file: "frontend/src/components/image/registry/index.tsx",
    markers: [
      "RemoteDataTable",
      "useRemoteTableState",
      "apiListKaniko",
      "globalSearch",
      "enableSorting: false",
    ],
  },
  {
    name: "Model Downloads",
    file: "frontend/src/components/model/model-downloads-page.tsx",
    markers: [
      "useRemoteTableState",
      "apiListModelDownloadsPage",
      "DataTablePagination",
      "totalItems={total}",
    ],
  },
  {
    name: "Operation Logs",
    file: "frontend/src/routes/admin/operation-logs/index.tsx",
    markers: ["RemoteDataTable", "getOperationLogsPaged", "id: 'createdAt'", "globalSearch"],
  },
  {
    name: "Cronjob Records",
    file: "frontend/src/routes/admin/cronjobs/-components/cronjob-records-table.tsx",
    markers: [
      "RemoteDataTable",
      "apiAdminCronJobRecordPage",
      "useRemoteTableState",
      "remoteFacets: true",
    ],
  },
  {
    name: "Cluster Resources",
    file: "frontend/src/routes/admin/cluster/resources/index.tsx",
    markers: [
      "RemoteDataTable",
      "apiResourceListPaged",
      "globalSearch",
      "remoteFacets: true",
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
