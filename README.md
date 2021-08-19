# orbs-sync

List, fetch and import CircleCI's orbs with dependencies in mind.
Designed to sync orbs between CircleCI.com and self-hosted CircleCI server instances.

Codes are hugely depending on / deriving from [CircleCI's CLI](https://github.com/CircleCI-Public/circleci-cli).

# Quick-start

1.  Download one of [convenient pre-built executables](https://github.com/circle-makotom/orbs-sync/releases) and extract it accordingly.
2.  `./orbs-sync sync --src-token YOUR_PERSONAL_TOKEN_FOR_CIRCLECI_DOTCOM --dst-host https://YOUR-CIRCLECI-SERVER-HOSTNAME.example.com --dst-token YOUR_PERSONAL_TOKEN_FOR_CIRCLECI_SERVER`

Then your CircleCI server instance should have all of CircleCI's official orbs in it. It may take as much as 5 minutes to sync all the orbs, depending on your location and the speed of your Internet connection.

# Deep dive

## Simply run from the source code

```
go run main.go
```

Basically running pre-built executables is recommended, unless you are actively editing the source codes, because `go run` effectively builds an executable every time you call it.

## Build your own executable

E.g. if you are to build an executable for Linux on x86-64 (AMD64), you would run:

```
GOOS_LIST_OVERRIDE=("linux") GOARCH_LIST_OVERRIDE=("amd64") ./build-and-pack-all.sh
```

Refer to [the official Go documentation](https://golang.org/doc/install/source#environment) for valid combinations of `GOOS` and `GOARCH`.

Note that the shell script depends on Zip, tar and gzip for packaging.

## More commands

There are multiple commands that are effectively working under the `sync` command.

- `collect` - crawl CircleCI's orbs registry and collect all the available and visible public orbs.
- `resolve-dependencies` - sort orbs based on dependencies; orbs installable without dependencies come first.
- `bulk-import` - import multiple orbs at once in the order of the given list.

See `./orbs-sync help` for details to run these commands separately.

# Technical notes

- The slowest part will be `bulk-import`. We need to import each version of each orb one-by-one, while we can fetch multiple versions of multiple orbs in bulk.

  - This is why `sync` takes account of orbs already available on the destination instance.

- The `collect` command collects _all_ the public orbs which are available and visible.

  - Note that it does not recognize hidden orbs and private orbs by default.

    - Due to this nature the dependency resolver would say that some orbs have unresolvable dependencies if those orbs are depending on hidden/private orbs.
    - You should be able to tune this nature by passing customized `--must-include` arguments.
      - Note that the argument can be specified multiple times to specify multiple orbs.

  - Due to technical limitations, it is not easy to collect _some_/selected versions of orbs.

    - This is why `sync` collects everything, then filter out those already on the destination instance. Implementing filtering mechanisms need complex codes with small benefits in terms of execution speed.

  - For this reason this programme can emit thousands of API requests in a short period, causing heavy loads for CircleCI. Do not abuse this, otherwise you can be banned!

- The programme can consume around 1 GiB of memory. This consumption happens as fetched orbs are loaded onto RAM.

- `--slow` option is recommended to run `collect`/`sync` with `--include-uncertified` enabled. Some third-party orbs are very big with lots of updates, and the fast strategy can hit API limitations in response sizes quite easily.

  - Note that `sync --include-uncertified --slow` may take as much as 3 hours to complete.
