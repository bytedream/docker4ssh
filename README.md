# docker4ssh - docker containers and more via ssh

**docker4ssh** is an ssh server that can create new docker containers and re-login into existing ones.

<p align="center">
  <a href="https://github.com/ByteDream/docker4ssh">
    <img src="https://img.shields.io/github/languages/code-size/ByteDream/docker4ssh?style=flat-square" alt="Code size">
  </a>
  <a href="https://github.com/ByteDream/docker4ssh/commits">
    <img src="https://img.shields.io/github/last-commit/ByteDream/docker4ssh?style=flat-square" alt="Latest commit">
  </a>
  <a href="https://github.com/ByteDream/docker4ssh/releases/latest">
    <img src="https://img.shields.io/github/downloads/ByteDream/docker4ssh/total?style=flat-square" alt="Download Badge">
  </a>
  <a href="https://github.com/ByteDream/docker4ssh/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/ByteDream/docker4ssh?style=flat-square" alt="License">
  </a>
  <a href="https://github.com/ByteDream/docker4ssh/releases/latest">
    <img src="https://img.shields.io/github/v/release/ByteDream/docker4ssh?style=flat-square" alt="Release">
  </a>
  <a href="https://discord.gg/gUWwekeNNg">
    <img src="https://img.shields.io/discord/915659846836162561?label=discord&style=flat-square" alt="Discord">
  </a>
</p>

<p align="center">
  <a href="#-features">✨ Features</a>
  ·
  <a href="#%EF%B8%8F-installation">⌨ Installation</a>
  ·
  <a href="#-usage">🖋️ Usage</a>
  ·
  <a href="#-license">⚖ License</a>
</p>

**Visit the [wiki](https://github.com/ByteDream/docker4ssh/wiki) to get more information and detailed usage instructions**

## ✨ Features
- Create containers by images (e.g. `ubuntu:21.04@server`)
- Create specific containers for specific usernames with [profiles](https://github.com/ByteDream/docker4ssh/wiki/Configuration-Files#profileconf)
- Containers are configurable from within
- Re-login into existing containers
- Full use of the docker api (unlike [docker2ssh](https://github.com/moul/ssh2docker), which uses the cli, which theoretically could cause code injection)
- Highly configurable [settings](https://github.com/ByteDream/docker4ssh/wiki/Configuration-Files#docker4sshconf)

## ⌨️ Installation

For every install method your OS **must** be linux and docker has to be installed.

- From the latest release (currently only x64 architecture supported)

  Replace the `RELEASE` value in the following oneliner with the latest release [version](https://github.com/ByteDream/docker4ssh/releases/latest).
  ```shell
  $ RELEASE=<latest version> curl -L https://github.com/ByteDream/docker4ssh/releases/download/v$RELEASE/docker4ssh-$RELEASE.tar.gz | tar -xvzf - -C /
- Building from source

    Before start installing, make sure you have to following things ready:
    - [Go](https://go.dev/) installed
    - [Rust](https://www.rust-lang.org/) installed
    - [Make](https://www.gnu.org/software/make/) installed - optional, but highly recommended since we use `make` in the further instructions
    
    To install docker4ssh, just execute the following commands
    ```shell
    $ git clone https://github.com/ByteDream/docker4ssh
    $ cd docker4ssh
    $ make build
    $ make install
    ```

- Install it from the [AUR](https://aur.archlinux.org/packages/docker4ssh/) (if you're using arch or an arch based distro)
  ```shell
  $ yay -S docker4ssh
  ```

## 🖋 Usage

To start the docker4ssh server, simply type
```shell
$ docker4ssh start
```

The default port for the ssh server is 2222, if you want to change it take a look at the [config file](https://github.com/ByteDream/docker4ssh/wiki/docker4ssh.conf).
Dynamic profile generation is enabled by default, so you can start right away.
Type the following to generate a new ubuntu container and connect to it:
```shell
$ ssh -p 2222 ubuntu:latest@127.0.0.1
```
You will get a password prompt then where you can type in anything since by default any password is correct.
If you typed in a password, the docker container gets created and the ssh connection is "redirected" to the containers' tty:
```shell
ubuntu:latest@127.0.0.1's password: 
┌───Container────────────────┐
│ Container ID: e0f3d48217da │
│ Network Mode: Host         │
│ Configurable: true         │
│ Run Level:    User         │
│ Exit After:                │
│ Keep On Exit: false        │
└──────────────Information───┘
root@e0f3d48217da:/#
```

For further information, visit the [wiki](https://github.com/ByteDream/docker4ssh/wiki).

## ⚖ License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for more details.