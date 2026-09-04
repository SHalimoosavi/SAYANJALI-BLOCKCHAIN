# ADR 0007 --- Production P2P Architecture

**Status:** Accepted for architecture direction; wire details remain
open\
**Date:** 2026-09-04

## Decision

The existing Python HTTP P2P layer remains the reference/prototype. The
future Go production network will use a formally specified long-running
peer protocol.

Required message families: - HELLO / HELLO_ACK - GET_PEERS / PEERS -
GET_HEADERS / HEADERS - GET_BLOCKS / BLOCKS - NEW_BLOCK -
NEW_TRANSACTION - REJECT

The final framing, transport security, request IDs, limits, timeouts and
negotiation rules must be specified before production implementation.

## Constraint

P2P must never implement independent consensus rules.
