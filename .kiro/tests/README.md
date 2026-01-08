# Property-Based Tests for Kubernetes API OIDC Authentication

This directory contains property-based tests that validate universal correctness properties of the Kubernetes API OIDC authentication system.

## Test Organization

Tests are organized by feature area:
- `auth/` - Authentication-related properties
- `rbac/` - Authorization and RBAC properties  
- `bootstrap/` - Bootstrap and system resilience properties
- `cli/` - AphexCLI contract properties

## Running Tests

```bash
# Run all property tests
go test ./...

# Run specific test suite
go test ./auth
go test ./rbac
go test ./bootstrap
go test ./cli
```

## Property Test Guidelines

Each property test:
- References its design document property number
- Validates universal correctness across generated inputs
- Runs minimum 100 iterations
- Uses descriptive test names with property references
