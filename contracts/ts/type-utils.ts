/**
 * Tiny type-level helpers used to assert that hand-written zod schemas
 * infer to the same shape as the OpenAPI-generated types.
 *
 * `Equals<A, B>` evaluates to `true` iff `A` and `B` are structurally
 * identical (not merely assignable) — the `<T>() => T extends X ? 1 : 2`
 * trick forces TypeScript to compare the normalized types.
 *
 * `Expect<T extends true>` is a carrier type that lives at module scope;
 * instantiating it with a non-`true` value is a compile error, which is
 * how the assertions in `schemas.ts` surface mismatches.
 */

export type Equals<A, B> =
  (<T>() => T extends A ? 1 : 2) extends
  (<T>() => T extends B ? 1 : 2)
    ? true
    : false;

export type Expect<T extends true> = T;
