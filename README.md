# Launchpad

### From Code to Kubernetes in One Step

[![Join Discord](https://img.shields.io/discord/903306922852245526?color=7389D8&label=discord&logo=discord&logoColor=ffffff)](https://discord.gg/jetify) ![License: Apache 2.0](https://img.shields.io/github/license/jetpack-io/devbox) [![version](https://img.shields.io/github/v/release/jetpack-io/launchpad?color=green&label=version&sort=semver)](https://github.com/jetify-com/launchpad/releases) [![tests](https://github.com/jetify-com/launchpad/actions/workflows/release.yaml/badge.svg)](https://github.com/jetify-com/launchpad/actions/workflows/release.yaml?branch=main)

## What is it?

[Launchpad](https://www.jetify.com/launchpad) is a command-line tool that lets you easily create applications on Kubernetes.

In practice, Launchpad works similar to Heroku or Vercel, except everything is on Kubernetes.

## Demo

The example below initializes a web project with `launchpad init`, and deploys to a local Kubernetes cluster with `launchpad up`:

![screen cast](https://www.jetify.com/assets/image/launchpad-docker-desktop-k.svg)

## Installing Launchpad

In addition to installing Launchpad itself, you will need to install `docker` since Launchpad depends on it:

1. Install [Docker Desktop](https://www.docker.com/get-started/).

2. Install Launchpad:

   ```sh
   curl -fsSL https://get.jetify.com/launchpad | bash
   ```

## Benefits

### A Zero Ops workflow

Launchpad builds any image, publishes it to your Docker Registry, and deploys it to Kubernetes in one step. No need to manually build and push your image, setup your kube-context, or write long pages of Kubernetes YAML.

### A Heroku-like experience on your own Kubernetes cluster

Ever wonder how you'd graduate from Heroku or a single EC2 machine to Kubernetes without going through a painful setup again? Faint not! With Launchpad, no manual migrations are required. In fact, developers can deploy and run their applications without needing to learn Kubernetes.

### Secret management built-in

Secrets are tied to your launchpad projects, so they can be shared and updated securely by your team.

## Quickstart: deploy to your Docker Desktop Kubernetes cluster

In this quickstart, we’ll deploy a cron job to your local Docker Desktop Kubernetes cluster.

1. Open a terminal in a new empty folder called `launchpad/`.

2. Enable [Kubernetes on Docker Desktop](https://docs.docker.com/desktop/kubernetes/)

3. Initialize Launchpad in `launchpad/`:

   ```bash
   > launchpad init
   ```

   You will see the following questions:

   ```
   ? What is the name of this project? // Press <Enter> to use the default name
   ? What type of service you would like to add to this project? // Choose `Cron Job`
   ? To which cluster do you want to deploy this project? // Choose `docker-desktop`
   ```

   This creates a `launchpad.yaml` file in the current directory. You should commit it to source control.

4. Your `launchpad.yaml` file should now look like this:

   ```yaml
      configVersion: 0.1.2
      projectId: ...
      name: app
      cluster: docker-desktop
      services:
        app-cron:
          type: cron
          image: busybox:latest
          command: [/bin/sh, -c, date; echo Hello from Launchpad]
          schedule: '* * * * *'
   ```

5. Start a new deployment to Kubernetes:

   ```bash
   launchpad up
   ```

6. Wait for a minute, and see the cron job in action:

   ```bash
   > kubectl get pods
   > kubectl logs <pod_name>
   ```

7. Clean up:

   ```bash
   launchpad down
   ```

## Additional commands

`launchpad help` - see all commands

`launchpad auth` - create a user, login, or logout (login required)

`launchpad env` - manage environment variables and secrets (login required)

`launchpad cluster` - create a cluster, list your clusters (login required)


## Join our Developer Community

- Chat with us by joining the [Jetify's Discord Server](https://discord.gg/jetify) – we have a #launchpad channel dedicated to this project.
- File bug reports and feature requests using [Github Issues](https://github.com/jetify-com/launchpad/issues)
- Follow us on [Jetpack's Twitter](https://twitter.com/jetify_com) for product updates

## Contributing

Launchpad is an open-core project so contributions are always welcome. Please read [our contributing guide](CONTRIBUTING.md) before submitting pull requests.
