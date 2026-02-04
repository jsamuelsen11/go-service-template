@integration @errors
Feature: Error Handling
  As a service consumer
  I want the service to handle errors gracefully
  So that I can understand what went wrong and respond appropriately

  Background:
    Given the service is running

  @smoke @not-found
  Scenario: Service returns 404 for unknown endpoints
    When I request GET "/unknown-endpoint-that-does-not-exist"
    Then the response status should be 404

  @smoke @method-not-allowed
  Scenario: Service returns 405 for wrong HTTP method on health endpoint
    When I request GET "/-/live"
    Then the response status should be 200

  @validation
  Scenario: Build info contains required fields
    When I request GET "/-/build"
    Then the response status should be 200
    And the response should contain "version"
    And the response should contain "commit"
    And the response should contain "buildTime"

  @metrics @prometheus
  Scenario: Metrics endpoint includes standard Go metrics
    When I request GET "/-/metrics"
    Then the response status should be 200
    And the response should contain "go_goroutines"
    And the response should contain "go_memstats_alloc_bytes"

  @health @readiness
  Scenario: Readiness returns health check details
    When I request GET "/-/ready"
    Then the response status should be 200
    And the response should contain "status"

  @health @liveness @graceful
  Scenario: Liveness probe is always available
    When I request GET "/-/live"
    Then the response status should be 200
    And the response should contain "ok"

  @resilience @timeout
  Scenario: Service responds within acceptable time
    When I request GET "/-/live"
    Then the response status should be 200
