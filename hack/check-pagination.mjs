#!/usr/bin/env node

import { pathToFileURL } from "node:url";

const DEFAULT_BASE_URL = "http://localhost:8088/api/v1";
const DEFAULT_PAGE_SIZE = 2;
const DEFAULT_SEARCH = "test";
const OUT_OF_RANGE_PAGE = 999_999;
const REQUEST_TIMEOUT_MS = 10_000;

export function buildChecks({
  pageSize = DEFAULT_PAGE_SIZE,
  search = DEFAULT_SEARCH,
} = {}) {
  return [
    {
      name: "JobTemplate",
      listPath: "jobtemplate",
      params: { search, sort: "-createdAt" },
    },
    {
      name: "VCJob",
      listPath: "vcjobs/all",
      facetsPath: "vcjobs/all/facets",
      facetKey: "status",
      facetValue: "Running",
      params: { days: "-1", status: ["Running"], search, sort: "-createdAt" },
    },
    {
      name: "ApprovalOrder",
      listPath: "approvalorder/page",
      params: { search, sort: "-createdAt" },
    },
    {
      name: "GPU Analysis",
      listPath: "admin/gpu-analysis/page",
      params: { search, sort: "-CreatedAt" },
      admin: true,
    },
    {
      name: "EMIAS",
      listPath: "aijobs/page",
      facetsPath: "aijobs/page/facets",
      facetKey: "status",
      facetValue: "Completed",
      params: {
        days: "-1",
        job_type: "training",
        status: ["Completed"],
        search,
        sort: "-createdAt",
      },
    },
  ].map((check) => ({ ...check, pageSize }));
}

function joinURL(baseURL, path) {
  return `${baseURL.replace(/\/$/, "")}/${path.replace(/^\//, "")}`;
}

function addParams(url, params) {
  for (const [key, value] of Object.entries(params)) {
    if (Array.isArray(value)) {
      for (const item of value) url.searchParams.append(key, String(item));
    } else if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  }
  return url;
}

function makeRequestURL(baseURL, path, params) {
  return addParams(new URL(joinURL(baseURL, path)), params);
}

async function fetchJSON(url, token, timeoutMs) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(url, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      signal: controller.signal,
    });
    const rawBody = await response.text();
    let body;
    try {
      body = rawBody ? JSON.parse(rawBody) : undefined;
    } catch {
      body = undefined;
    }
    return { response, body, rawBody };
  } finally {
    clearTimeout(timer);
  }
}

function classifyHTTPFailure(status) {
  if (status === 401 || status === 403) return "blocked";
  return "failed";
}

function readPage(body) {
  const page = body?.data;
  if (!body || typeof body !== "object" || !page || typeof page !== "object") {
    throw new Error("response is missing data");
  }
  if (!Array.isArray(page.items)) throw new Error("data.items is not an array");
  for (const key of ["total", "page", "page_size"]) {
    if (!Number.isSafeInteger(page[key]) || page[key] < 0) {
      throw new Error(`data.${key} is not a non-negative integer`);
    }
  }
  if (page.page_size < 1) throw new Error("data.page_size must be positive");
  return page;
}

function readFacets(body) {
  const facets = body?.data?.facets;
  if (!facets || typeof facets !== "object" || Array.isArray(facets)) {
    throw new Error("response is missing data.facets");
  }
  for (const [name, items] of Object.entries(facets)) {
    if (!Array.isArray(items))
      throw new Error(`data.facets.${name} is not an array`);
    for (const item of items) {
      if (
        item?.value === undefined ||
        !Number.isSafeInteger(item?.count) ||
        item.count < 0
      ) {
        throw new Error(`data.facets.${name} contains an invalid item`);
      }
    }
  }
  return facets;
}

function validateQuery(url, expected) {
  for (const [key, value] of Object.entries(expected)) {
    const expectedValues = Array.isArray(value)
      ? value.map(String)
      : [String(value)];
    const actualValues = url.searchParams.getAll(key);
    if (actualValues.join("\u0000") !== expectedValues.join("\u0000")) {
      throw new Error(
        `query ${key}=${JSON.stringify(actualValues)}, expected ${JSON.stringify(expectedValues)}`,
      );
    }
  }
}

function itemIdentity(item) {
  if (!item || typeof item !== "object") return JSON.stringify(item);
  for (const key of [
    "id",
    "ID",
    "uuid",
    "uid",
    "name",
    "jobName",
    "job_name",
    "orderNo",
    "order_no",
  ]) {
    if (item[key] !== undefined && item[key] !== null) {
      return `${key}:${String(item[key])}`;
    }
  }
  return JSON.stringify(item);
}

function assertNoPageOverlap(firstItems, secondItems) {
  const secondIds = new Set(secondItems.map(itemIdentity));
  const duplicates = firstItems
    .map(itemIdentity)
    .filter((id) => secondIds.has(id));
  if (duplicates.length > 0) {
    throw new Error(
      `page 1 and page 2 contain duplicate items: ${duplicates.join(", ")}`,
    );
  }
}

function assertFacetMatchesTotal(facets, facetKey, facetValue, total) {
  const items = facets[facetKey];
  if (!Array.isArray(items)) throw new Error(`missing facet ${facetKey}`);
  const matches = items.filter(
    (item) => String(item.value) === String(facetValue),
  );
  if (matches.length > 1) {
    throw new Error(`facet ${facetKey} contains duplicate value ${facetValue}`);
  }
  const facetCount = matches[0]?.count ?? 0;
  if (facetCount !== total) {
    throw new Error(
      `facet ${facetKey}=${facetValue} has count ${facetCount}, list total is ${total}`,
    );
  }
}

async function checkRequest({ url, token, timeoutMs, validate, read }) {
  let result;
  try {
    result = await fetchJSON(url, token, timeoutMs);
  } catch (error) {
    const message =
      error?.name === "AbortError"
        ? `timeout after ${timeoutMs}ms`
        : error.message;
    return { status: "blocked", message, url: url.toString() };
  }

  const { response, body, rawBody } = result;
  if (!response.ok) {
    return {
      status: classifyHTTPFailure(response.status),
      message: `HTTP ${response.status}${rawBody ? `: ${rawBody.slice(0, 160)}` : ""}`,
      url: url.toString(),
    };
  }

  try {
    validate?.(url);
    const value = read(body);
    return { status: "passed", value, url: url.toString() };
  } catch (error) {
    return { status: "failed", message: error.message, url: url.toString() };
  }
}

function checkPageShape(page, expectedPage, pageSize) {
  if (page.page !== expectedPage) {
    throw new Error(`data.page=${page.page}, expected ${expectedPage}`);
  }
  if (page.page_size !== pageSize) {
    throw new Error(`data.page_size=${page.page_size}, expected ${pageSize}`);
  }
  if (page.items.length > pageSize) {
    throw new Error(
      `data.items has ${page.items.length} items, expected at most ${pageSize}`,
    );
  }
  if (page.total < 0) throw new Error("data.total must be non-negative");
}

function failedCheck(check, message, requests) {
  return { ...check, status: "failed", message, requests };
}

function blockedOrFailedCheck(check, request, requests) {
  return {
    ...check,
    status: request.status,
    message: request.message,
    requests,
  };
}

async function checkOne(check, options) {
  const { baseURL, token, timeoutMs } = options;
  const common = { page_size: check.pageSize, ...check.params };
  const requests = [];

  const baselineParams = { ...common };
  delete baselineParams.search;
  const baseline = await checkRequest({
    url: makeRequestURL(baseURL, check.listPath, {
      ...baselineParams,
      page: 1,
    }),
    token,
    timeoutMs,
    validate: (url) => validateQuery(url, { ...baselineParams, page: 1 }),
    read: (body) => {
      const page = readPage(body);
      checkPageShape(page, 1, check.pageSize);
      return page;
    },
  });
  requests.push(baseline);
  if (baseline.status !== "passed")
    return blockedOrFailedCheck(check, baseline, requests);

  const first = await checkRequest({
    url: makeRequestURL(baseURL, check.listPath, { ...common, page: 1 }),
    token,
    timeoutMs,
    validate: (url) => validateQuery(url, { ...common, page: 1 }),
    read: (body) => {
      const page = readPage(body);
      checkPageShape(page, 1, check.pageSize);
      return page;
    },
  });
  requests.push(first);
  if (first.status !== "passed")
    return blockedOrFailedCheck(check, first, requests);

  try {
    if (first.value.total > baseline.value.total) {
      throw new Error(
        `search total ${first.value.total} exceeds unfiltered total ${baseline.value.total}`,
      );
    }
  } catch (error) {
    return failedCheck(check, error.message, requests);
  }

  const second = await checkRequest({
    url: makeRequestURL(baseURL, check.listPath, { ...common, page: 2 }),
    token,
    timeoutMs,
    validate: (url) => validateQuery(url, { ...common, page: 2 }),
    read: (body) => {
      const page = readPage(body);
      checkPageShape(page, 2, check.pageSize);
      return page;
    },
  });
  requests.push(second);
  if (second.status !== "passed")
    return blockedOrFailedCheck(check, second, requests);

  try {
    if (second.value.total !== first.value.total) {
      throw new Error(
        `page 2 total ${second.value.total} differs from page 1 total ${first.value.total}`,
      );
    }
    assertNoPageOverlap(first.value.items, second.value.items);
  } catch (error) {
    return failedCheck(check, error.message, requests);
  }

  const changedPageSize = check.pageSize === 1 ? 2 : check.pageSize - 1;
  const resized = await checkRequest({
    url: makeRequestURL(baseURL, check.listPath, {
      ...common,
      page_size: changedPageSize,
      page: 1,
    }),
    token,
    timeoutMs,
    validate: (url) =>
      validateQuery(url, { ...common, page_size: changedPageSize, page: 1 }),
    read: (body) => {
      const page = readPage(body);
      checkPageShape(page, 1, changedPageSize);
      return page;
    },
  });
  requests.push(resized);
  if (resized.status !== "passed")
    return blockedOrFailedCheck(check, resized, requests);
  try {
    if (resized.value.total !== first.value.total) {
      throw new Error(
        `page_size change altered total: ${first.value.total} -> ${resized.value.total}`,
      );
    }
  } catch (error) {
    return failedCheck(check, error.message, requests);
  }

  const outOfRange = await checkRequest({
    url: makeRequestURL(baseURL, check.listPath, {
      ...common,
      page: OUT_OF_RANGE_PAGE,
    }),
    token,
    timeoutMs,
    validate: (url) =>
      validateQuery(url, { ...common, page: OUT_OF_RANGE_PAGE }),
    read: (body) => {
      const page = readPage(body);
      checkPageShape(page, OUT_OF_RANGE_PAGE, check.pageSize);
      if (page.items.length !== 0)
        throw new Error("out-of-range page returned items");
      return page;
    },
  });
  requests.push(outOfRange);
  if (outOfRange.status !== "passed")
    return blockedOrFailedCheck(check, outOfRange, requests);

  if (check.facetsPath) {
    const facets = await checkRequest({
      url: makeRequestURL(baseURL, check.facetsPath, common),
      token,
      timeoutMs,
      validate: (url) => validateQuery(url, common),
      read: (body) => {
        const value = readFacets(body);
        if (check.facetKey) {
          assertFacetMatchesTotal(
            value,
            check.facetKey,
            check.facetValue,
            first.value.total,
          );
        }
        return value;
      },
    });
    requests.push(facets);
    if (facets.status !== "passed")
      return blockedOrFailedCheck(check, facets, requests);
  }

  return {
    ...check,
    status: "passed",
    message: `${requests.length} read-only requests and behavior checks passed`,
    requests,
  };
}

export async function runAcceptance({
  baseURL = process.env.CRATER_API_BASE_URL || DEFAULT_BASE_URL,
  token = process.env.CRATER_AUTH_TOKEN || process.env.CRATER_TOKEN || "",
  userToken = process.env.CRATER_USER_AUTH_TOKEN || token,
  adminToken = process.env.CRATER_ADMIN_AUTH_TOKEN || "",
  pageSize = Number(
    process.env.CRATER_ACCEPTANCE_PAGE_SIZE || DEFAULT_PAGE_SIZE,
  ),
  search = process.env.CRATER_ACCEPTANCE_SEARCH || DEFAULT_SEARCH,
  timeoutMs = REQUEST_TIMEOUT_MS,
  log = console.log,
} = {}) {
  if (!Number.isInteger(pageSize) || pageSize < 1 || pageSize > 200) {
    throw new Error("pageSize must be an integer between 1 and 200");
  }

  const checks = buildChecks({ pageSize, search });
  log(`Crater pagination acceptance: ${baseURL}`);
  log(
    `Read-only mode: page_size=${pageSize}, search=${JSON.stringify(search)}`,
  );
  log(
    `Token roles: user=${userToken ? "provided" : "missing"}, admin=${adminToken ? "provided" : "missing"}`,
  );

  const results = [];
  for (const check of checks) {
    const checkToken = check.admin ? adminToken || token : userToken;
    const result = await checkOne(check, {
      baseURL,
      token: checkToken,
      timeoutMs,
    });
    result.tokenRole = check.admin ? "admin" : "user";
    results.push(result);
    log(
      `[${result.status.toUpperCase()}] ${result.name} (${result.tokenRole} token): ${result.message || "no details"}`,
    );
    for (const request of result.requests || []) log(`  ${request.url}`);
  }

  const passed = results.filter((result) => result.status === "passed").length;
  const blocked = results.filter(
    (result) => result.status === "blocked",
  ).length;
  const failed = results.filter((result) => result.status === "failed").length;
  log(`Summary: ${passed} passed, ${blocked} blocked, ${failed} failed`);
  return { results, passed, blocked, failed };
}

function parseArgs(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--base-url") options.baseURL = argv[++index];
    else if (arg === "--token") options.token = argv[++index];
    else if (arg === "--user-token") options.userToken = argv[++index];
    else if (arg === "--admin-token") options.adminToken = argv[++index];
    else if (arg === "--page-size") options.pageSize = Number(argv[++index]);
    else if (arg === "--search") options.search = argv[++index];
    else if (arg === "--timeout") options.timeoutMs = Number(argv[++index]);
    else if (arg === "--allow-blocked") options.allowBlocked = true;
    else if (arg === "--help" || arg === "-h") options.help = true;
    else throw new Error(`unknown argument: ${arg}`);
  }
  return options;
}

function printHelp() {
  console.log(`Usage: node hack/check-pagination.mjs [options]

Read-only checks for JobTemplate, VCJob, ApprovalOrder, GPU Analysis, and EMIAS.

Options:
  --base-url URL       API root, default: $CRATER_API_BASE_URL or ${DEFAULT_BASE_URL}
  --token TOKEN        Fallback Bearer token, prefer $CRATER_AUTH_TOKEN
  --user-token TOKEN   Token for user-scoped checks, prefer $CRATER_USER_AUTH_TOKEN
  --admin-token TOKEN  Token for GPU/admin checks, prefer $CRATER_ADMIN_AUTH_TOKEN
  --page-size N        Page size, 1-200, default: 2
  --search TEXT        Search value, default: test
  --timeout MS         Per-request timeout, default: ${REQUEST_TIMEOUT_MS}
  --allow-blocked      Exit 0 when only auth/network blockers remain
  -h, --help           Show this help

Example:
  CRATER_USER_AUTH_TOKEN=... CRATER_ADMIN_AUTH_TOKEN=... node hack/check-pagination.mjs \\
    --base-url https://crater.act.buaa.edu.cn/api/v1
`);
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    printHelp();
    return;
  }
  const result = await runAcceptance(options);
  if (result.failed > 0 || (result.blocked > 0 && !options.allowBlocked)) {
    process.exitCode = 1;
  }
}

const entrypoint =
  process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url;
if (entrypoint) {
  main().catch((error) => {
    console.error(`ERROR: ${error.message}`);
    process.exitCode = 1;
  });
}
