package jflags

// jflags contain structs for flags of cobra commands
//
// package name: we use jflags for the package name so that variables can
// continue being named "flags" without having a conflict with the package name.
//
// Naming Convention:
//
// 1. Command Flag Structs
// They contain the full set of flags of a cobra-command.
//
// These are named as {name}CmdFlags. For example, upCmdFlags.
//
// 2. Embedded Flag Structs
// These structs are embedded inside the Command Flag Structs. They let us
// compose flags into multiple commands.
//
// These are named as {name}Flags. For example, deployFlags are composed or
// embedded into upCmdFlags and devCmdFlags.
//
