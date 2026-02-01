/**
 * k6 Load Test for Health Endpoints
 *
 * Usage:
 *   k6 run test/load/k6/health.js
 *   k6 run --env BASE_URL=http://staging:8080 test/load/k6/health.js
 *   k6 run --env SCENARIO=steady_state test/load/k6/health.js
 *
 * Scenarios:
 *   - steady_state: Constant 100 RPS for 5 minutes
 *   - ramp_up: Gradual increase from 0 to 500 RPS
 *   - spike: Baseline load with sudden spike
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const livenessLatency = new Trend('liveness_latency', true);
const buildInfoLatency = new Trend('build_info_latency', true);

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  scenarios: {
    // Steady state: constant load for extended period
    steady_state: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 50,
      maxVUs: 150,
      exec: 'steadyStateTest',
      startTime: '0s',
    },

    // Ramp up: gradually increasing load
    ramp_up: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 500,
      stages: [
        { duration: '30s', target: 50 },
        { duration: '1m', target: 200 },
        { duration: '30s', target: 500 },
        { duration: '1m', target: 500 },
        { duration: '30s', target: 0 },
      ],
      exec: 'rampUpTest',
      startTime: '6m',
    },

    // Spike: sudden burst of traffic
    spike: {
      executor: 'ramping-arrival-rate',
      startRate: 100,
      timeUnit: '1s',
      preAllocatedVUs: 200,
      maxVUs: 1000,
      stages: [
        { duration: '1m', target: 100 },   // Baseline
        { duration: '10s', target: 1000 }, // Spike up
        { duration: '30s', target: 1000 }, // Hold spike
        { duration: '10s', target: 100 },  // Spike down
        { duration: '1m', target: 100 },   // Recovery
      ],
      exec: 'spikeTest',
      startTime: '10m',
    },
  },

  thresholds: {
    // Overall HTTP metrics
    http_req_duration: ['p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],

    // Custom metrics
    errors: ['rate<0.01'],
    liveness_latency: ['p(95)<50', 'p(99)<100'],
    build_info_latency: ['p(95)<50', 'p(99)<100'],
  },

  // Output configuration
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
};

// Test functions - only test endpoints that don't hit external dependencies
export function steadyStateTest() {
  testLiveness();
  testBuildInfo();
}

export function rampUpTest() {
  testLiveness();
  testBuildInfo();
}

export function spikeTest() {
  testLiveness();
  testBuildInfo();
}

// Default function for simple runs
export default function () {
  testLiveness();
  testBuildInfo();
  sleep(0.1);
}

// Liveness endpoint test
function testLiveness() {
  const url = `${BASE_URL}/-/live`;
  const res = http.get(url, {
    tags: { endpoint: 'liveness' },
  });

  livenessLatency.add(res.timings.duration);

  const success = check(res, {
    'liveness status is 200': (r) => r.status === 200,
    'liveness has status field': (r) => r.body.includes('status'),
    'liveness response time < 100ms': (r) => r.timings.duration < 100,
  });

  errorRate.add(!success);
}

// Build info endpoint test
function testBuildInfo() {
  const url = `${BASE_URL}/-/build`;
  const res = http.get(url, {
    tags: { endpoint: 'build' },
  });

  buildInfoLatency.add(res.timings.duration);

  const success = check(res, {
    'build status is 200': (r) => r.status === 200,
    'build has version': (r) => r.body.includes('version'),
    'build has commit': (r) => r.body.includes('commit'),
    'build response time < 100ms': (r) => r.timings.duration < 100,
  });

  errorRate.add(!success);
}

// Metrics endpoint test
export function testMetrics() {
  const url = `${BASE_URL}/-/metrics`;
  const res = http.get(url, {
    tags: { endpoint: 'metrics' },
  });

  const success = check(res, {
    'metrics status is 200': (r) => r.status === 200,
    'metrics has prometheus format': (r) => r.body.includes('go_goroutines'),
  });

  errorRate.add(!success);
}

// Setup function - runs once at the beginning
export function setup() {
  console.log(`Testing against: ${BASE_URL}`);

  // Verify service is running
  const res = http.get(`${BASE_URL}/-/live`);
  if (res.status !== 200) {
    throw new Error(`Service is not running at ${BASE_URL}`);
  }

  return { baseUrl: BASE_URL };
}

// Teardown function - runs once at the end
export function teardown(data) {
  console.log(`Load test completed against: ${data.baseUrl}`);
}
