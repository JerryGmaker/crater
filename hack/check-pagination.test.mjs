import assert from "node:assert/strict";
import { createServer } from "node:http";
import { after, before, describe, it } from "node:test";

import { runAcceptance } from "./check-pagination.mjs";

const routes = new Set([
  "/api/v1/jobtemplate",
  "/api/v1/vcjobs/all",
  "/api/v1/vcjobs/all/facets",
  "/api/v1/approvalorder/page",
  "/api/v1/admin/gpu-analysis/page",
  "/api/v1/images/kaniko/page",
  "/api/v1/admin/images/kaniko/page",
  "/api/v1/aijobs/page",
  "/api/v1/aijobs/page/facets",
]);

let server;
let baseURL;
const requests = [];

before(async () => {
  server = createServer((request, response) => {
    const url = new URL(request.url, "http://localhost");
    requests.push({ url, authorization: request.headers.authorization });

    if (!routes.has(url.pathname)) {
      response.writeHead(404, { "content-type": "application/json" });
      response.end(JSON.stringify({ message: "not found" }));
      return;
    }

    response.setHeader("content-type", "application/json");
    if (url.pathname.endsWith("/facets")) {
      const status = url.searchParams.get("status");
      response.end(
        JSON.stringify({
          code: 0,
          data: { facets: { status: [{ value: status, count: 3 }] } },
        }),
      );
      return;
    }

    const page = Number(url.searchParams.get("page"));
    const pageSize = Number(url.searchParams.get("page_size"));
    const hasSearch = url.searchParams.get("search") === "fixture";
    const items = page === 999999 ? [] : [{ id: `${url.pathname}-${page}` }];
    response.end(
      JSON.stringify({
        code: 0,
        data: { items, total: hasSearch ? 3 : 10, page, page_size: pageSize },
      }),
    );
  });

  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  baseURL = `http://127.0.0.1:${server.address().port}/api/v1`;
});

after(async () => {
  await new Promise((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
});

describe("pagination acceptance script", () => {
  it("checks all seven representative APIs with read-only query requests", async () => {
    const result = await runAcceptance({
      baseURL,
      userToken: "user-token",
      adminToken: "admin-token",
      pageSize: 2,
      search: "fixture",
      timeoutMs: 1000,
      log: () => {},
    });

    assert.equal(result.passed, 7);
    assert.equal(result.blocked, 0);
    assert.equal(result.failed, 0);
    assert.equal(requests.length, 37);
    assert.ok(
      requests.every(({ url, authorization }) =>
        url.pathname.startsWith("/api/v1/admin/")
          ? authorization === "Bearer admin-token"
          : authorization === "Bearer user-token",
      ),
    );

    const pageRequests = requests.filter(
      ({ url }) => !url.pathname.endsWith("/facets"),
    );
    assert.ok(pageRequests.every(({ url }) => url.searchParams.has("page")));
    assert.ok(
      pageRequests.filter(
        ({ url }) => url.searchParams.get("page_size") === "2",
      ).length > 0,
    );
    assert.ok(pageRequests.some(({ url }) => !url.searchParams.has("search")));
    assert.ok(
      pageRequests
        .filter(({ url }) => url.searchParams.has("search"))
        .every(({ url }) => url.searchParams.get("search") === "fixture"),
    );
    assert.ok(pageRequests.every(({ url }) => url.searchParams.has("sort")));
    assert.ok(
      pageRequests.some(({ url }) => url.searchParams.get("page_size") === "1"),
    );
  });
});
