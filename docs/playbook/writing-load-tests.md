# Writing Load Tests

This guide covers writing load tests using k6 for performance and stress testing.

## Overview

k6 is a modern load testing tool that uses JavaScript for test scripts. It's great for:

- Load testing (sustained traffic)
- Stress testing (finding limits)
- Spike testing (sudden traffic bursts)
- Soak testing (prolonged usage)

**Location:** `test/load/k6/`

---

## Script Structure

**File:** `test/load/k6/health.js`

```javascript
import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// Custom metrics
const errorRate = new Rate("errors");
const latency = new Trend("endpoint_latency", true);

// Configuration
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// Test options
export const options = {
  scenarios: {
    // Define test scenarios
  },
  thresholds: {
    // Define pass/fail criteria
  },
};

// Default test function
export default function () {
  // Test logic
}

// Setup (runs once at start)
export function setup() {
  // Verify service is running
}

// Teardown (runs once at end)
export function teardown(data) {
  // Cleanup
}
```

---

## Test Scenarios

### Constant Load (Steady State)

Test sustained traffic at a fixed rate:

```javascript
export const options = {
  scenarios: {
    steady_state: {
      executor: "constant-arrival-rate",
      rate: 100, // 100 requests per second
      timeUnit: "1s",
      duration: "5m",
      preAllocatedVUs: 50, // Start with 50 virtual users
      maxVUs: 150, // Scale up to 150 if needed
    },
  },
};
```

### Ramp Up/Down

Gradually increase then decrease load:

```javascript
export const options = {
  scenarios: {
    ramp_up: {
      executor: "ramping-arrival-rate",
      startRate: 10,
      timeUnit: "1s",
      preAllocatedVUs: 100,
      maxVUs: 500,
      stages: [
        { duration: "30s", target: 50 }, // Ramp to 50 RPS
        { duration: "1m", target: 200 }, // Ramp to 200 RPS
        { duration: "30s", target: 500 }, // Ramp to 500 RPS
        { duration: "1m", target: 500 }, // Hold at 500 RPS
        { duration: "30s", target: 0 }, // Ramp down
      ],
    },
  },
};
```

### Spike Test

Sudden burst of traffic:

```javascript
export const options = {
  scenarios: {
    spike: {
      executor: "ramping-arrival-rate",
      startRate: 100,
      timeUnit: "1s",
      preAllocatedVUs: 200,
      maxVUs: 1000,
      stages: [
        { duration: "1m", target: 100 }, // Baseline
        { duration: "10s", target: 1000 }, // Spike up
        { duration: "30s", target: 1000 }, // Hold spike
        { duration: "10s", target: 100 }, // Spike down
        { duration: "1m", target: 100 }, // Recovery
      ],
    },
  },
};
```

### Multiple Scenarios

Run different scenarios sequentially:

```javascript
export const options = {
    scenarios: {
        steady_state: {
            executor: 'constant-arrival-rate',
            rate: 100,
            duration: '5m',
            startTime: '0s',       // Start immediately
            exec: 'steadyStateTest',
        },
        ramp_up: {
            executor: 'ramping-arrival-rate',
            startRate: 10,
            stages: [...],
            startTime: '6m',       // Start after steady_state
            exec: 'rampUpTest',
        },
        spike: {
            executor: 'ramping-arrival-rate',
            stages: [...],
            startTime: '10m',      // Start after ramp_up
            exec: 'spikeTest',
        },
    },
};

// Scenario-specific test functions
export function steadyStateTest() { /* ... */ }
export function rampUpTest() { /* ... */ }
export function spikeTest() { /* ... */ }
```

---

## Thresholds (Pass/Fail Criteria)

```javascript
export const options = {
  thresholds: {
    // HTTP metrics
    http_req_duration: ["p(95)<200", "p(99)<500"], // 95th < 200ms, 99th < 500ms
    http_req_failed: ["rate<0.01"], // < 1% failure rate

    // Custom metrics
    errors: ["rate<0.01"], // < 1% error rate
    endpoint_latency: ["p(95)<100", "p(99)<200"], // Custom latency metric
  },
};
```

### Common Threshold Patterns

| Metric              | Threshold   | Meaning                   |
| ------------------- | ----------- | ------------------------- |
| `http_req_duration` | `p(95)<200` | 95th percentile < 200ms   |
| `http_req_failed`   | `rate<0.01` | Failure rate < 1%         |
| `http_reqs`         | `rate>100`  | At least 100 RPS          |
| `errors`            | `count<10`  | Less than 10 total errors |

---

## Test Functions

### Basic HTTP Test

```javascript
import http from "k6/http";
import { check } from "k6";

export default function () {
  const url = `${BASE_URL}/-/live`;

  const res = http.get(url, {
    tags: { endpoint: "liveness" }, // Tag for filtering
  });

  // Assertions
  const success = check(res, {
    "status is 200": (r) => r.status === 200,
    "response has status": (r) => r.body.includes("status"),
    "latency < 100ms": (r) => r.timings.duration < 100,
  });

  // Record to custom metric
  errorRate.add(!success);
  latency.add(res.timings.duration);
}
```

### POST with JSON Body

```javascript
import http from "k6/http";
import { check } from "k6";

export default function () {
  const payload = JSON.stringify({
    name: "Test User",
    email: "test@example.com",
  });

  const params = {
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + __ENV.API_TOKEN,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/users`, payload, params);

  check(res, {
    created: (r) => r.status === 201,
    "has id": (r) => JSON.parse(r.body).id !== undefined,
  });
}
```

### Test Multiple Endpoints

```javascript
import http from "k6/http";
import { check, group } from "k6";

export default function () {
  group("Health Endpoints", () => {
    const liveRes = http.get(`${BASE_URL}/-/live`);
    check(liveRes, { "liveness ok": (r) => r.status === 200 });

    const readyRes = http.get(`${BASE_URL}/-/ready`);
    check(readyRes, { "readiness ok": (r) => r.status === 200 });
  });

  group("API Endpoints", () => {
    const quoteRes = http.get(`${BASE_URL}/api/v1/quotes/random`);
    check(quoteRes, { "quote ok": (r) => r.status === 200 });
  });
}
```

---

## Custom Metrics

```javascript
import { Counter, Gauge, Rate, Trend } from "k6/metrics";

// Counter: Cumulative count
const requestCount = new Counter("requests_total");

// Gauge: Point-in-time value
const activeUsers = new Gauge("active_users");

// Rate: Percentage (0-1)
const errorRate = new Rate("error_rate");

// Trend: Distribution (for percentiles)
const latency = new Trend("latency_ms", true);

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/resource`);

  // Record metrics
  requestCount.add(1);
  errorRate.add(res.status !== 200);
  latency.add(res.timings.duration);
}
```

---

## Setup and Teardown

```javascript
// Runs once at the beginning (before VUs start)
export function setup() {
  console.log(`Testing against: ${BASE_URL}`);

  // Verify service is available
  const res = http.get(`${BASE_URL}/-/live`);
  if (res.status !== 200) {
    throw new Error(`Service not available at ${BASE_URL}`);
  }

  // Return data to share with default function
  return {
    baseUrl: BASE_URL,
    testStartTime: new Date().toISOString(),
  };
}

// Runs once at the end (after all VUs finish)
export function teardown(data) {
  console.log(`Test completed. Started at: ${data.testStartTime}`);
}

// data from setup() is passed to default function
export default function (data) {
  const res = http.get(`${data.baseUrl}/-/live`);
  // ...
}
```

---

## Running Load Tests

### Basic Run

```bash
# Run with task
task test:load

# Or directly with k6
k6 run test/load/k6/health.js
```

### With Environment Variables

```bash
# Different target
k6 run --env BASE_URL=http://staging:8080 test/load/k6/health.js

# With authentication
k6 run --env API_TOKEN=secret123 test/load/k6/api.js
```

### Run Specific Scenario

```bash
k6 run --scenario steady_state test/load/k6/health.js
```

### Output Formats

```bash
# JSON output
k6 run --out json=results.json test/load/k6/health.js

# InfluxDB (for Grafana dashboards)
k6 run --out influxdb=http://localhost:8086/k6 test/load/k6/health.js

# Cloud (k6 Cloud)
k6 cloud test/load/k6/health.js
```

### Quick Smoke Test

```bash
# Override options for quick test
k6 run --vus 10 --duration 30s test/load/k6/health.js
```

---

## Reading Results

```text
     scenarios: (100.00%) 1 scenario, 150 max VUs, 5m30s max duration
                data_received..................: 45 MB  150 kB/s
                data_sent......................: 12 MB  40 kB/s
                http_req_blocked...............: avg=1.2ms    p(95)=5ms
                http_req_duration..............: avg=15ms     p(95)=45ms   p(99)=120ms
                http_req_failed................: 0.05%  ✓ 150   ✗ 299850
                http_reqs......................: 300000 1000/s
                iteration_duration.............: avg=16ms     p(95)=50ms
                iterations.....................: 300000 1000/s
                vus............................: 100    min=50  max=150
                vus_max........................: 150    min=150 max=150

     ✓ liveness status is 200
     ✓ liveness has status field
     ✓ liveness response time < 100ms
```

### Key Metrics

| Metric              | Description                |
| ------------------- | -------------------------- |
| `http_req_duration` | Total request time         |
| `http_req_failed`   | Failed requests percentage |
| `http_reqs`         | Total requests and rate    |
| `vus`               | Virtual users (concurrent) |
| `iterations`        | Completed test iterations  |

---

## Complete Example

**File:** `test/load/k6/api.js`

```javascript
import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// Custom metrics
const errorRate = new Rate("errors");
const apiLatency = new Trend("api_latency", true);

// Configuration
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
  scenarios: {
    api_load: {
      executor: "ramping-arrival-rate",
      startRate: 10,
      timeUnit: "1s",
      preAllocatedVUs: 50,
      maxVUs: 200,
      stages: [
        { duration: "1m", target: 50 },
        { duration: "3m", target: 100 },
        { duration: "1m", target: 50 },
      ],
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<500", "p(99)<1000"],
    http_req_failed: ["rate<0.05"],
    errors: ["rate<0.05"],
    api_latency: ["p(95)<400"],
  },
};

export function setup() {
  const res = http.get(`${BASE_URL}/-/live`);
  if (res.status !== 200) {
    throw new Error("Service not available");
  }
  return { baseUrl: BASE_URL };
}

export default function (data) {
  // Test random quote endpoint
  const res = http.get(`${data.baseUrl}/api/v1/quotes/random`, {
    tags: { endpoint: "quotes" },
  });

  const success = check(res, {
    "status 200": (r) => r.status === 200,
    "has content": (r) => r.body.includes("content"),
    "latency ok": (r) => r.timings.duration < 500,
  });

  errorRate.add(!success);
  apiLatency.add(res.timings.duration);

  sleep(0.1); // Small delay between requests
}

export function teardown(data) {
  console.log(`Load test completed against: ${data.baseUrl}`);
}
```

---

## Checklist

- [ ] Script in `test/load/k6/`
- [ ] `BASE_URL` configurable via environment
- [ ] Appropriate scenario (constant, ramping, spike)
- [ ] Thresholds defined for pass/fail
- [ ] Setup verifies service availability
- [ ] Checks validate response correctness
- [ ] Custom metrics for important measurements

---

## Related Documentation

- [Writing Benchmark Tests](./writing-benchmark-tests.md)
- [k6 Documentation](https://k6.io/docs/)
