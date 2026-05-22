# awsc - AWS Console in your Terminal

A full-screen terminal UI for AWS, inspired by [K9s](https://k9scli.io/). Navigate your AWS resources like you would in the console, but from your terminal with vim-style keybindings.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ awsc  profile:production  region:eu-west-1 > ec2                            │
│ <Enter> details  <t> terminate  <r> reboot  <s> stop  <S> start  <R> refresh│
├─────────────────────────────────────────────────────────────────────────────┤
│ NAME              INSTANCE ID          STATE    TYPE       PRIVATE IP        │
│ web-server-1      i-0abc123def456789   running  t3.medium  10.0.1.100       │
│ api-server-1      i-0def456abc789012   running  m5.large   10.0.2.50        │
│ db-primary        i-0789012def345abc   stopped  r5.xlarge  10.0.3.10        │
├─────────────────────────────────────────────────────────────────────────────┤
│ : ec2                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Installation

```bash
# From source
go install github.com/tpriestnall/awsc/cmd/awsc@latest

# Or build locally
make build
./bin/awsc
```

## Usage

```bash
# Use default profile and region
awsc

# Specify profile and region
awsc --profile production --region eu-west-1

# Print version
awsc --version
```

## Navigation

awsc uses a K9s-style command system:

| Command | Action |
|---------|--------|
| `:ec2` | Go to EC2 instances |
| `:ecr` | Go to ECR repositories |
| `:sg` | Go to Security Groups |
| `:vpc` | Go to VPCs |
| `:subnet` | Go to Subnets |
| `:services` | Go to service list (home) |
| `:region=us-east-1` | Switch region |
| `:region` | Open region picker |
| `:profile=staging` | Switch AWS profile |
| `:quit` / `q` | Quit |

## Keyboard Shortcuts

### Global
| Key | Action |
|-----|--------|
| `:` | Open command bar |
| `/` | Open filter bar |
| `Esc` | Go back / cancel |
| `q` | Quit |

### EC2 Instances
| Key | Action |
|-----|--------|
| `Enter` | View instance details |
| `t` | Terminate instance |
| `r` | Reboot instance |
| `s` | Stop instance |
| `S` | Start instance |
| `R` | Refresh list |

### EC2 Instance Detail
| Key | Action |
|-----|--------|
| `Enter` | Open selected security group |
| `v` | Navigate to VPC |
| `n` | Navigate to subnet |
| `t` | Terminate instance |
| `r` | Reboot instance |

### ECR Repositories
| Key | Action |
|-----|--------|
| `Enter` | View images |
| `d` | Delete repository |
| `R` | Refresh list |

### Security Groups
| Key | Action |
|-----|--------|
| `Enter` | View rules detail |
| `v` | Navigate to VPC |
| `R` | Refresh list |

## Filtering

The filter bar (`/`) supports flexible expressions:

```
# Simple text search (matches across all fields)
/web-server

# Field-specific filters
/name contains web
/state = running
/type = t3.micro
/vpc_id = vpc-abc123
/az = us-east-1a

# Tag-based filtering
/tag:env = production
/tag:team contains platform

# Operators: =, !=, contains, starts_with, ends_with
/name starts_with api
/private_ip starts_with 10.0.1
```

## Service Coverage

### Fully Supported
| Service | Features |
|---------|----------|
| **EC2** | List, detail, terminate, reboot, stop, start, filter, security group drill-down, VPC/subnet navigation |
| **ECR** | List repos, view images, delete images, delete repos, create repos |
| **Security Groups** | List all, view rules (ingress/egress), add/remove rules, VPC navigation |
| **VPC** | List all, view details, navigate to subnets |
| **Subnets** | List all (optionally by VPC), navigate to VPC |

### Next Priority (Roadmap)
| Priority | Service | Planned Features |
|----------|---------|-----------------|
| 1 | **S3** | List buckets, browse objects, upload/download, presigned URLs |
| 2 | **ECS** | Clusters, services, tasks, exec into containers |
| 3 | **CloudWatch** | Log groups, live log tailing, metrics |
| 4 | **Lambda** | List functions, invoke, view logs, update code |
| 5 | **RDS** | Instances, clusters, snapshots, parameter groups |
| 6 | **IAM** | Users, roles, policies, access analyzer |
| 7 | **EKS** | Clusters, node groups, integrate with kubeconfig |
| 8 | **Route53** | Hosted zones, record sets |
| 9 | **ELB/ALB** | Load balancers, target groups, listeners |
| 10 | **CloudFormation** | Stacks, events, drift detection |

## Architecture

```
cmd/awsc/              - Entry point, flag parsing, wiring
internal/
  aws/                 - AWS client management
    ec2/               - EC2 service layer (API calls, data models)
    ecr/               - ECR service layer
  config/              - App configuration (profiles, regions)
  navigation/          - Route stack, command registry
  ui/
    app.go             - Main TUI application shell
    components/        - Reusable widgets (header, omnibox)
    views/             - Resource-specific views
      services/        - Service listing (home)
      ec2/             - EC2 list + detail views
      ecr/             - ECR list + image views
      sg/              - Security Groups view
      vpc/             - VPC view
      subnet/          - Subnet view
    theme/             - Color palette and styling
```

## Design Principles

1. **K9s-like UX** - If you know K9s, you know awsc. Same mental model.
2. **Everything navigable** - Any resource reference can be selected to drill into it.
3. **Non-destructive by default** - Destructive operations require confirmation.
4. **Fast feedback** - Background loading with status indicators.
5. **Filterable everything** - Every list supports the same filter syntax.
6. **Single binary** - No runtime dependencies, just your AWS credentials.

## Requirements

- Go 1.24+
- AWS credentials configured (`~/.aws/credentials`, env vars, or IAM role)
- A terminal that supports 256 colors

## Development

```bash
# Run tests
make test

# Run with race detector
make test

# Build
make build

# Cross-compile for all platforms
make build-all

# Format code
make fmt

# Run linter
make lint
```

## License

MIT
