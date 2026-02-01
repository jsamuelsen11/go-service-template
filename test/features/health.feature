@smoke @health
Feature: Health Endpoints
  As a Kubernetes operator
  I want health endpoints
  So that I can monitor service health and make routing decisions

  Background:
    Given the service is running

  @smoke @liveness
  Scenario: Liveness probe returns OK
    When I request GET "/-/live"
    Then the response status should be 200
    And the response should contain "status"
    And the response should contain "ok"

  @smoke @readiness
  Scenario: Readiness probe returns OK when healthy
    When I request GET "/-/ready"
    Then the response status should be 200
    And the response should contain "status"

  @smoke @build
  Scenario: Build info returns version information
    When I request GET "/-/build"
    Then the response status should be 200
    And the response should contain "version"
    And the response should contain "commit"
    And the response should contain "buildTime"
    And the response should contain "goVersion"

  @smoke @metrics
  Scenario: Metrics endpoint returns Prometheus format
    When I request GET "/-/metrics"
    Then the response status should be 200
    And the response should contain "go_goroutines"
    And the response should contain "go_gc_duration_seconds"
