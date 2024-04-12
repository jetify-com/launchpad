# Contributing

When contributing to this repository, please describe the change you wish to make via a related issue, or a pull request.

Please note we have a [code of conduct](CODE_OF_CONDUCT.md), please follow it in all your interactions with the project.

## Setting Up Development Environment

Before making any changes to the source code (documentation excluded) make sure you have installed all the required tools. We use Devbox to manage our development environment.

### Prerequisites

-   Install [Devbox](https://github.com/jetify/devbox#installing-devbox).
-   Clone this repository:
    -   ```bash
          git clone git@github.com:jetify/launchpad.git go.jetify.com/launchpad
        ```
-   Use devbox to init your development environment:
    -   ```bash
          cd go.jetify.com/launchpad
        ```
    -   ```bash
          devbox shell
        ```

## Building and Testing

Launchpad is setup like a typical Go project. After installing the required tools and setting up your environment. You can make changes in the source code, build, and test your changes by following these steps:

1. Install dependencies:
    ```bash
    go install
    ```
2. Build Launchpad:
    ```bash
    go build -o ./dist/launchpad cmd/launchpad/main.go
    ```
    This will build an executable file.
3. Run and test Launchpad:
    ```bash
    ./dist/launchpad <your_test_command>
    ```

## Pull Request Process

1. Ensure any new feature or functionality also includes tests to verify its correctness.

2. Ensure any new dependency is also included in [go.mod](go.mod) file

3. Ensure any binary file as a result of build (e.g., `./launchpad`) are removed and/or excluded from tracking in git.

4. Update the [README.md](README.md) and/or docs with details of changes to the interface, this includes new environment
   variables, new commands, new flags, and useful file locations.

5. You may merge the Pull Request in once you have the sign-off of developers/maintainers, or if you
   do not have permission to do that, you may request the maintainers to merge it for you.

## Developer Certificate of Origin

By contributing to this project you agree to the [Developer Certificate of Origin](https://developercertificate.org/) (DCO) which was created by the Linux Foundation and is a simple statement that you, as a contributor, have the legal right to make the contribution. See the DCO description for details below:

> Developer Certificate of Origin
>
> Version 1.1
>
> Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
>
> Everyone is permitted to copy and distribute verbatim copies of this
> license document, but changing it is not allowed.
>
> Developer's Certificate of Origin 1.1
>
> By making a contribution to this project, I certify that:
>
> (a) The contribution was created in whole or in part by me and I

    have the right to submit it under the open source license
    indicated in the file; or

> (b) The contribution is based upon previous work that, to the best

    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

> (c) The contribution was provided directly to me by some other

    person who certified (a), (b) or (c) and I have not modified
    it.

> (d) I understand and agree that this project and the contribution

    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
