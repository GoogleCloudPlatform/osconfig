# E2E Test Timing Report (`TestOS*`)

**Data source:** `e2e_tests_v2/artifacts/timings-20261119t224950z-55b0135a.json`, `e2e_tests_v2/artifacts/junit.xml`

## Run Summary

| Metric | Value |
|---|---|
| Run ID | `20261119t224950z` |
| Suites / Cases | 19 suites / 119 cases (**117 PASS**, **2 FAIL**, 0 SKIP) |
| Concurrency & Polling Settings | `-parallel=10`, `-max_concurrent_vms=10`, `-poll_interval=10s` |
| Total Wall Time | **16,008.3s (266.8m / 4h 27m)** |
| Cumulative VM Time (sum of all 119 cases) | 55,876.3s (931.3m) |
| Sum of Per-Suite Max Durations | 15,763.6s (262.7m) |
| Average Effective Concurrency | 3.49 VMs (out of 10) |
| Inventory / Agent Ready Wait Share | 49.8% of total VM time (42.9% excluding `TestOSInventory`) |
| Cleanup Share | 16.6% of total VM time (129.6s avg in OS Policy suites vs 31.9s in other suites) |

---

## 1. Per-Suite and Per-Case Durations

### Suite Summary

| Suite | Cases | FAIL | Avg (s) | Max (s) | Cleanup Avg (s) |
|---|---:|---:|---:|---:|---:|
| `TestOSInventory` | 18 | 1 | 262.5 | 1090.5 | 31.7 |
| `TestOSPatchJobExecution` | 14 | 0 | 559.4 | 1228.7 | 35.1 |
| `TestOSPatchPrePostSteps` | 15 | 0 | 585.7 | 1407.8 | 35.9 |
| `TestOSPatchYum` | 10 | 0 | 411.9 | 879.4 | 23.7 |
| `TestOSPatchZypperExcludes` | 3 | 0 | 243.4 | 286.2 | 29.6 |
| `TestOSPatchZypperExclusivePatches` | 3 | 0 | 228.4 | 242.2 | 26.8 |
| `TestOSPolicyFileResourceLinux` | 11 | 0 | 399.9 | 716.8 | 128.8 |
| `TestOSPolicyFileResourceWindows` | 3 | 0 | 854.0 | 1251.4 | 126.4 |
| `TestOSPolicyPackageResourceApt` | 4 | 0 | 378.1 | 415.1 | 151.1 |
| `TestOSPolicyPackageResourceDeb` | 4 | 0 | 389.7 | 405.6 | 152.1 |
| `TestOSPolicyPackageResourceGooGet` | 3 | 0 | 806.7 | 1193.0 | 131.0 |
| `TestOSPolicyPackageResourceMsi` | 3 | 0 | 885.7 | 1358.7 | 125.3 |
| `TestOSPolicyPackageResourceRpm` | 7 | 0 | 447.6 | 844.0 | 122.5 |
| `TestOSPolicyPackageResourceYum` | 5 | 0 | 521.4 | 1010.0 | 115.3 |
| `TestOSPolicyPackageResourceZypper` | 2 | 0 | 425.8 | 445.7 | 124.5 |
| `TestOSPolicyRepositoryResourceApt` | 4 | 0 | 303.1 | 326.3 | 149.4 |
| `TestOSPolicyRepositoryResourceGooGet` | 3 | 0 | 867.6 | 1314.7 | 129.4 |
| `TestOSPolicyRepositoryResourceYum` | 5 | 1 | 547.1 | 963.0 | 115.0 |
| `TestOSPolicyRepositoryResourceZypper` | 2 | 0 | 373.9 | 384.5 | 119.2 |

### Breakdown Across All 119 Cases

#### `TestOSInventory` (18 cases, avg 262.5s, max 1090.5s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1090.5 | 13.6 | 1033.6 | 0.0 | 43.2 |
| `rhel-8` | PASS | 504.0 | 13.0 | 467.2 | 0.0 | 23.8 |
| `windows-2022` | PASS | 475.1 | 33.8 | 409.1 | 0.0 | 32.2 |
| `windows-2019` | PASS | 436.0 | 14.9 | 388.9 | 0.0 | 32.3 |
| `rhel-9` | PASS | 310.5 | 14.0 | 274.3 | 0.0 | 22.2 |
| `sles-12` | PASS | 220.3 | 22.5 | 165.5 | 0.0 | 32.4 |
| `sles-15` | PASS | 212.2 | 13.8 | 166.2 | 0.0 | 32.2 |
| `ubuntu-2404-lts` | PASS | 188.8 | 13.1 | 111.9 | 0.0 | 63.8 |
| `ubuntu-2204-lts` | PASS | 177.2 | 11.8 | 101.6 | 0.0 | 63.7 |
| `rocky-linux-8` | PASS | 160.7 | 14.1 | 123.8 | 0.0 | 22.8 |
| `debian-12` | PASS | 147.8 | 11.7 | 82.0 | 0.0 | 54.1 |
| `rocky-linux-9` | PASS | 141.8 | 24.6 | 94.4 | 0.0 | 22.8 |
| `opensuse-leap-15` | PASS | 141.2 | 13.3 | 104.2 | 0.0 | 23.7 |
| `centos-stream-9` | PASS | 140.8 | 14.1 | 105.1 | 0.0 | 21.5 |
| `cos-stable` | PASS | 129.7 | 23.0 | 83.9 | 0.0 | 22.8 |
| `cos-beta` | PASS | 128.5 | 23.6 | 71.2 | 0.0 | 33.6 |
| `cos-dev` | PASS | 116.7 | 13.1 | 81.4 | 0.0 | 22.2 |
| `debian-11` | **FAIL** | 3.1 | 1.3 | 0.0 | 0.0 | 1.7 |

#### `TestOSPatchJobExecution` (14 cases, avg 559.4s, max 1228.7s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1228.7 | 12.2 | 933.7 | 249.3 | 33.5 |
| `windows-2022` | PASS | 795.7 | 12.9 | 469.3 | 278.3 | 35.2 |
| `rhel-8` | PASS | 793.6 | 23.9 | 346.4 | 401.2 | 22.2 |
| `windows-2019` | PASS | 784.2 | 12.4 | 459.0 | 278.8 | 34.0 |
| `rhel-9` | PASS | 735.4 | 24.1 | 255.0 | 432.6 | 23.7 |
| `rocky-linux-9` | PASS | 601.7 | 13.3 | 133.7 | 431.8 | 22.9 |
| `ubuntu-2204-lts` | PASS | 569.1 | 14.3 | 448.7 | 41.7 | 64.4 |
| `rocky-linux-8` | PASS | 530.4 | 14.3 | 153.9 | 339.3 | 23.0 |
| `centos-stream-9` | PASS | 487.5 | 12.3 | 113.1 | 339.1 | 23.1 |
| `ubuntu-2404-lts` | PASS | 398.8 | 24.2 | 92.6 | 216.3 | 65.7 |
| `debian-12` | PASS | 244.6 | 14.0 | 83.1 | 93.0 | 54.5 |
| `sles-15` | PASS | 241.3 | 23.3 | 142.3 | 41.7 | 34.0 |
| `sles-12` | PASS | 231.3 | 23.6 | 132.4 | 41.6 | 33.7 |
| `opensuse-leap-15` | PASS | 188.7 | 13.0 | 112.0 | 41.6 | 22.1 |

#### `TestOSPatchPrePostSteps` (15 cases, avg 585.7s, max 1407.8s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1407.8 | 12.5 | 1061.3 | 310.5 | 23.4 |
| `windows-2019` | PASS | 886.7 | 12.2 | 500.0 | 340.9 | 33.6 |
| `windows-2022` | PASS | 863.7 | 22.7 | 437.2 | 370.6 | 33.2 |
| `rocky-linux-9` | PASS | 684.0 | 24.1 | 142.3 | 494.7 | 22.8 |
| `rhel-8` | PASS | 682.5 | 13.3 | 398.1 | 247.5 | 23.7 |
| `ubuntu-2404-lts` | PASS | 674.0 | 12.7 | 111.8 | 494.6 | 54.8 |
| `rhel-9` | PASS | 650.2 | 22.3 | 203.2 | 402.5 | 22.2 |
| `rocky-linux-8` | PASS | 561.0 | 22.2 | 143.1 | 371.7 | 24.0 |
| `centos-stream-9` | PASS | 520.0 | 22.4 | 102.9 | 371.6 | 23.1 |
| `sles-15` | PASS | 357.9 | 14.4 | 184.4 | 124.3 | 34.8 |
| `sles-12` | PASS | 322.9 | 24.0 | 162.6 | 103.3 | 33.0 |
| `debian-12` | PASS | 316.1 | 12.0 | 93.0 | 155.3 | 55.8 |
| `ubuntu-2204-lts` | PASS | 293.4 | 13.0 | 121.9 | 103.3 | 55.2 |
| `debian-13` | PASS | 291.2 | 12.4 | 91.5 | 124.0 | 63.4 |
| `opensuse-leap-15` | PASS | 274.6 | 34.3 | 111.9 | 93.5 | 34.9 |

#### `TestOSPatchYum` (10 cases, avg 411.9s, max 879.4s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `excludes/rhel-8` | PASS | 879.4 | 13.1 | 442.1 | 401.3 | 22.9 |
| `exclusive_packages/rhel-8` | PASS | 609.5 | 12.4 | 441.0 | 123.2 | 32.9 |
| `excludes/rhel-9` | PASS | 587.7 | 11.6 | 245.6 | 308.2 | 22.3 |
| `excludes/rocky-linux-9` | PASS | 539.6 | 106.2 | 132.2 | 278.2 | 23.0 |
| `excludes/rocky-linux-8` | PASS | 356.4 | 13.0 | 135.1 | 186.1 | 22.2 |
| `exclusive_packages/rhel-9` | PASS | 273.2 | 13.5 | 195.9 | 41.8 | 22.1 |
| `exclusive_packages/rocky-linux-8` | PASS | 243.3 | 23.9 | 155.2 | 41.5 | 22.7 |
| `exclusive_packages/rocky-linux-9` | PASS | 223.9 | 23.8 | 135.3 | 41.5 | 23.2 |
| `excludes/centos-stream-9` | PASS | 214.2 | 34.1 | 114.7 | 42.1 | 23.4 |
| `exclusive_packages/centos-stream-9` | PASS | 191.6 | 12.6 | 114.9 | 41.8 | 22.2 |

#### `TestOSPatchZypperExcludes` (3 cases, avg 243.4s, max 286.2s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `sles-15` | PASS | 286.2 | 12.4 | 166.2 | 72.9 | 34.6 |
| `sles-12` | PASS | 263.9 | 24.5 | 165.2 | 41.7 | 32.6 |
| `opensuse-leap-15` | PASS | 180.0 | 23.3 | 93.5 | 41.7 | 21.5 |

#### `TestOSPatchZypperExclusivePatches` (3 cases, avg 228.4s, max 242.2s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `sles-15` | PASS | 242.2 | 13.7 | 153.5 | 41.6 | 33.4 |
| `sles-12` | PASS | 241.6 | 13.0 | 163.8 | 41.8 | 23.0 |
| `opensuse-leap-15` | PASS | 201.5 | 12.1 | 123.9 | 41.7 | 23.8 |

#### `TestOSPolicyFileResourceLinux` (11 cases, avg 399.9s, max 716.8s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `rhel-8` | PASS | 716.8 | 12.8 | 437.8 | 152.9 | 113.2 |
| `rhel-9` | PASS | 444.1 | 12.6 | 225.5 | 91.8 | 114.2 |
| `sles-15` | PASS | 403.1 | 12.1 | 173.9 | 91.9 | 125.2 |
| `ubuntu-2404-lts` | PASS | 374.5 | 22.8 | 112.5 | 91.9 | 147.4 |
| `ubuntu-2204-lts` | PASS | 373.3 | 12.5 | 113.2 | 92.1 | 155.5 |
| `rocky-linux-8` | PASS | 372.8 | 13.0 | 142.5 | 91.9 | 125.4 |
| `debian-13` | PASS | 353.1 | 13.2 | 102.5 | 91.9 | 145.5 |
| `opensuse-leap-15` | PASS | 351.3 | 11.7 | 133.9 | 91.9 | 113.8 |
| `debian-12` | PASS | 344.1 | 13.5 | 92.5 | 92.0 | 146.0 |
| `centos-stream-9` | PASS | 342.9 | 13.0 | 123.4 | 91.9 | 114.6 |
| `rocky-linux-9` | PASS | 323.2 | 11.9 | 103.7 | 92.0 | 115.6 |

#### `TestOSPolicyFileResourceWindows` (3 cases, avg 854.0s, max 1251.4s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1251.4 | 13.2 | 987.9 | 124.4 | 125.9 |
| `windows-2022` | PASS | 675.5 | 22.1 | 433.5 | 93.6 | 126.3 |
| `windows-2019` | PASS | 635.3 | 12.3 | 399.0 | 97.0 | 127.0 |

#### `TestOSPolicyPackageResourceApt` (4 cases, avg 378.1s, max 415.1s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `ubuntu-2204-lts` | PASS | 415.1 | 13.4 | 152.6 | 92.3 | 156.8 |
| `ubuntu-2404-lts` | PASS | 383.6 | 12.3 | 122.3 | 92.2 | 156.9 |
| `debian-12` | PASS | 362.3 | 22.8 | 101.7 | 92.1 | 145.7 |
| `debian-13` | PASS | 351.5 | 22.7 | 91.5 | 92.1 | 145.2 |

#### `TestOSPolicyPackageResourceDeb` (4 cases, avg 389.7s, max 405.6s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `ubuntu-2204-lts` | PASS | 405.6 | 12.9 | 112.0 | 122.6 | 158.1 |
| `ubuntu-2404-lts` | PASS | 404.1 | 13.1 | 112.1 | 122.5 | 156.4 |
| `debian-13` | PASS | 375.3 | 23.1 | 82.0 | 122.7 | 147.5 |
| `debian-12` | PASS | 373.7 | 13.0 | 91.6 | 122.7 | 146.4 |

#### `TestOSPolicyPackageResourceGooGet` (3 cases, avg 806.7s, max 1193.0s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1193.0 | 13.0 | 930.6 | 118.8 | 130.6 |
| `windows-2019` | PASS | 640.4 | 12.3 | 398.1 | 92.4 | 137.6 |
| `windows-2022` | PASS | 586.8 | 12.5 | 357.3 | 92.1 | 124.8 |

#### `TestOSPolicyPackageResourceMsi` (3 cases, avg 885.7s, max 1358.7s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1358.7 | 11.6 | 1076.2 | 145.6 | 125.2 |
| `windows-2022` | PASS | 653.6 | 13.7 | 391.8 | 122.6 | 125.4 |
| `windows-2019` | PASS | 644.8 | 25.4 | 402.0 | 92.2 | 125.1 |

#### `TestOSPolicyPackageResourceRpm` (7 cases, avg 447.6s, max 844.0s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `rhel-8` | PASS | 844.0 | 13.0 | 448.1 | 225.2 | 157.7 |
| `rhel-9` | PASS | 436.8 | 13.7 | 215.4 | 92.2 | 115.5 |
| `opensuse-leap-15` | PASS | 425.7 | 25.7 | 163.3 | 122.6 | 114.1 |
| `sles-15` | PASS | 414.4 | 23.3 | 174.0 | 92.3 | 124.8 |
| `rocky-linux-8` | PASS | 344.1 | 13.9 | 123.7 | 92.1 | 114.5 |
| `centos-stream-9` | PASS | 342.9 | 23.6 | 112.4 | 92.1 | 114.8 |
| `rocky-linux-9` | PASS | 325.1 | 14.2 | 102.8 | 92.1 | 116.0 |

#### `TestOSPolicyPackageResourceYum` (5 cases, avg 521.4s, max 1010.0s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `rhel-8` | PASS | 1010.0 | 23.3 | 747.4 | 123.4 | 115.9 |
| `rhel-9` | PASS | 486.1 | 54.5 | 223.9 | 92.3 | 115.4 |
| `rocky-linux-8` | PASS | 394.6 | 13.1 | 173.9 | 92.1 | 115.6 |
| `centos-stream-9` | PASS | 363.5 | 23.5 | 133.0 | 92.0 | 115.0 |
| `rocky-linux-9` | PASS | 352.9 | 22.6 | 123.6 | 92.1 | 114.7 |

#### `TestOSPolicyPackageResourceZypper` (2 cases, avg 425.8s, max 445.7s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `sles-15` | PASS | 445.7 | 12.7 | 204.9 | 98.8 | 129.3 |
| `opensuse-leap-15` | PASS | 405.8 | 24.3 | 143.1 | 118.8 | 119.7 |

#### `TestOSPolicyRepositoryResourceApt` (4 cases, avg 303.1s, max 326.3s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `ubuntu-2204-lts` | PASS | 326.3 | 23.8 | 0.0 | 155.0 | 147.5 |
| `debian-13` | PASS | 296.2 | 23.9 | 0.0 | 125.0 | 147.4 |
| `ubuntu-2404-lts` | PASS | 296.1 | 13.9 | 0.0 | 125.1 | 157.1 |
| `debian-12` | PASS | 293.9 | 23.7 | 0.0 | 124.4 | 145.7 |

#### `TestOSPolicyRepositoryResourceGooGet` (3 cases, avg 867.6s, max 1314.7s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `windows-2016` | PASS | 1314.7 | 12.9 | 1051.8 | 124.1 | 125.9 |
| `windows-2019` | PASS | 649.1 | 13.4 | 417.7 | 92.1 | 126.0 |
| `windows-2022` | PASS | 639.1 | 12.4 | 398.3 | 92.1 | 136.3 |

#### `TestOSPolicyRepositoryResourceYum` (5 cases, avg 547.1s, max 963.0s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `rhel-8` | PASS | 963.0 | 12.3 | 0.0 | 714.0 | 236.7 |
| `centos-stream-9` | **FAIL** | 924.3 | 12.9 | 0.0 | 887.2 | 24.1 |
| `rhel-9` | PASS | 343.8 | 13.6 | 0.0 | 214.6 | 115.6 |
| `rocky-linux-8` | PASS | 283.7 | 15.5 | 0.0 | 153.6 | 114.5 |
| `rocky-linux-9` | PASS | 220.9 | 13.2 | 0.0 | 123.6 | 84.1 |

#### `TestOSPolicyRepositoryResourceZypper` (2 cases, avg 373.9s, max 384.5s)

| Case (OS) | Status | Total (s) | Create VM (s) | Wait Ready/Inv (s) | Main Step(s) (s) | Cleanup (s) |
|---|---|---:|---:|---:|---:|---:|
| `sles-15` | PASS | 384.5 | 13.1 | 153.6 | 93.4 | 124.3 |
| `opensuse-leap-15` | PASS | 363.3 | 12.9 | 122.5 | 113.9 | 114.0 |

---

## 2. Average Duration by Step

Sorted by cumulative VM time:

| Step / Phase | Count | Avg (s) | p50 (s) | p95 (s) | Max (s) | Total (s) | Share of VM Time |
|---|---:|---:|---:|---:|---:|---:|---:|
| `wait for OS Config agent ready` | 45 | 238.2 | 153.5 | 500.0 | 1061.3 | 10,718.2 | 19.2% |
| `[cleanup]` | 119 | 77.9 | 63.4 | 156.8 | 236.7 | 9,266.4 | 16.6% |
| `wait for agent inventory` | 26 | 307.4 | 153.6 | 1051.8 | 1076.2 | 7,993.4 | 14.3% |
| `create os policy assignment` | 56 | 132.7 | 93.0 | 214.6 | 887.2 | 7,430.0 | 13.3% |
| `wait for initial package state in inventory` | 21 | 249.7 | 163.3 | 747.4 | 930.6 | 5,243.5 | 9.4% |
| `execute and await patch job with pre and post steps` | 15 | 273.9 | 310.5 | 494.7 | 494.7 | 4,108.4 | 7.4% |
| `wait for inventory` | 17 | 227.3 | 111.9 | 1033.6 | 1033.6 | 3,864.4 | 6.9% |
| `execute and await patch job` | 14 | 230.5 | 278.3 | 432.6 | 432.6 | 3,226.3 | 5.8% |
| `create VM` | 119 | 17.7 | 13.5 | 25.7 | 106.2 | 2,102.2 | 3.8% |
| `execute and await patch job with YUM settings` | 10 | 150.6 | 123.2 | 401.3 | 401.3 | 1,505.6 | 2.7% |
| `execute and await patch job with Zypper settings` | 3 | 52.1 | 41.7 | 72.9 | 72.9 | 156.3 | 0.3% |
| `execute and await patch job with Zypper exclusive patches` | 3 | 41.7 | 41.7 | 41.8 | 41.8 | 125.1 | 0.2% |
| `verify package installed via inventory` | 16 | 4.1 | 0.2 | 21.2 | 21.2 | 64.9 | 0.1% |
| `verify packages via inventory` | 14 | 3.4 | 0.2 | 22.4 | 22.4 | 47.5 | 0.1% |
| `verify packages installed via inventory` | 11 | 1.1 | 0.2 | 10.4 | 10.4 | 12.4 | <0.1% |
| `wait for os policy compliance report` | 55 | 0.2 | 0.2 | 0.7 | 0.8 | 11.7 | <0.1% |
| `[queue_wait]` | 119 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0% |
| `validate OsInfo` / `verify installed and available packages` | 17 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0 | 0.0% |

---

## 3. Top 20 Slowest Cases

| # | Test / Case | Status | Total (s) | Cleanup (s) | Slowest Step |
|---:|---|---|---:|---:|---|
| 1 | `TestOSPatchPrePostSteps/windows-2016` | PASS | **1407.8** | 23.4 | `wait for OS Config agent ready` (1061.3s) |
| 2 | `TestOSPolicyPackageResourceMsi/windows-2016` | PASS | **1358.7** | 125.2 | `wait for agent inventory` (1076.2s) |
| 3 | `TestOSPolicyRepositoryResourceGooGet/windows-2016` | PASS | **1314.7** | 125.9 | `wait for agent inventory` (1051.8s) |
| 4 | `TestOSPolicyFileResourceWindows/windows-2016` | PASS | **1251.4** | 125.9 | `wait for agent inventory` (987.9s) |
| 5 | `TestOSPatchJobExecution/windows-2016` | PASS | **1228.7** | 33.5 | `wait for OS Config agent ready` (933.7s) |
| 6 | `TestOSPolicyPackageResourceGooGet/windows-2016` | PASS | **1193.0** | 130.6 | `wait for initial package state in inventory` (930.6s) |
| 7 | `TestOSInventory/windows-2016` | PASS | **1090.5** | 43.2 | `wait for inventory` (1033.6s) |
| 8 | `TestOSPolicyPackageResourceYum/rhel-8` | PASS | **1010.0** | 115.9 | `wait for initial package state in inventory` (747.4s) |
| 9 | `TestOSPolicyRepositoryResourceYum/rhel-8` | PASS | **963.0** | 236.7 | `create os policy assignment` (693.2s) |
| 10 | `TestOSPolicyRepositoryResourceYum/centos-stream-9` | **FAIL** | **924.3** | 24.1 | `create os policy assignment` (887.2s) |
| 11 | `TestOSPatchPrePostSteps/windows-2019` | PASS | **886.7** | 33.6 | `wait for OS Config agent ready` (500.0s) |
| 12 | `TestOSPatchYum/excludes/rhel-8` | PASS | **879.4** | 22.9 | `wait for OS Config agent ready` (442.1s) |
| 13 | `TestOSPatchPrePostSteps/windows-2022` | PASS | **863.7** | 33.2 | `wait for OS Config agent ready` (437.2s) |
| 14 | `TestOSPolicyPackageResourceRpm/rhel-8` | PASS | **844.0** | 157.7 | `wait for initial package state in inventory` (448.1s) |
| 15 | `TestOSPatchJobExecution/windows-2022` | PASS | **795.7** | 35.2 | `wait for OS Config agent ready` (469.3s) |
| 16 | `TestOSPatchJobExecution/rhel-8` | PASS | **793.6** | 22.2 | `execute and await patch job` (401.2s) |
| 17 | `TestOSPatchJobExecution/windows-2019` | PASS | **784.2** | 34.0 | `wait for OS Config agent ready` (459.0s) |
| 18 | `TestOSPatchJobExecution/rhel-9` | PASS | **735.4** | 23.7 | `execute and await patch job` (432.6s) |
| 19 | `TestOSPolicyFileResourceLinux/rhel-8` | PASS | **716.8** | 113.2 | `wait for agent inventory` (437.8s) |
| 20 | `TestOSPatchPrePostSteps/rocky-linux-9` | PASS | **684.0** | 22.8 | `execute and await patch job with pre and post steps` (494.7s) |

