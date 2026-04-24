"use client";

import { Info } from "lucide-react";
import type { SettingsSchemaNode } from "@/lib/api/schemas";
import { hintForNode, labelForKey } from "@/lib/admin/settings/format";
import { cn } from "@/lib/utils";
import { FieldBoolean } from "./FieldBoolean";
import { FieldEnum } from "./FieldEnum";
import { FieldJson } from "./FieldJson";
import { FieldNumber } from "./FieldNumber";
import { FieldString } from "./FieldString";
import { FieldStringArray } from "./FieldStringArray";

export interface SettingFieldProps {
  node: SettingsSchemaNode;
  value: unknown;
  onChange: (v: unknown) => void;
  error?: string | null;
}

/**
 * Single setting row: label, hint/tooltip, widget, inline error. Picks
 * the widget based on node.type — the schema drives the UI.
 */
export function SettingField({ node, value, onChange, error }: SettingFieldProps) {
  const id = `setting-${node.key.replace(/\./g, "-")}`;
  const hint = hintForNode(node);
  const invalid = !!error;
  const disabled = node.readOnly === true;

  return (
    <div className="grid grid-cols-[minmax(0,14rem)_minmax(0,1fr)] items-start gap-3 py-2">
      <div className="flex items-start gap-1.5 pt-1.5">
        <label
          htmlFor={id}
          className="text-sm font-medium text-foreground"
        >
          {labelForKey(node.key)}
        </label>
        <span
          title={`${node.key}${hint ? `\n${hint}` : ""}`}
          aria-label={`${node.key} — ${hint || "no description"}`}
          className="inline-flex text-muted-foreground"
        >
          <Info className="h-3.5 w-3.5" aria-hidden="true" />
        </span>
      </div>
      <div className="flex flex-col gap-1">
        {renderWidget(node, id, value, onChange, invalid, disabled)}
        {hint ? (
          <p className={cn("text-xs text-muted-foreground")}>{hint}</p>
        ) : null}
        {error ? (
          <p className="text-xs text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <p className="text-[10px] uppercase tracking-wide text-muted-foreground/60">
          {node.key}
        </p>
      </div>
    </div>
  );
}

function renderWidget(
  node: SettingsSchemaNode,
  id: string,
  value: unknown,
  onChange: (v: unknown) => void,
  invalid: boolean,
  disabled: boolean,
) {
  switch (node.type) {
    case "boolean":
      return (
        <FieldBoolean
          id={id}
          value={value === true}
          disabled={disabled}
          onChange={onChange}
          label={node.key}
        />
      );
    case "enum":
      return (
        <FieldEnum
          id={id}
          value={typeof value === "string" ? value : ""}
          options={(node.enumValues ?? []).filter(
            (v): v is string => typeof v === "string",
          )}
          onChange={onChange}
          disabled={disabled}
          aria-invalid={invalid}
        />
      );
    case "integer":
      return (
        <FieldNumber
          id={id}
          value={typeof value === "number" ? value : ""}
          step={node.step ?? 1}
          min={node.minimum}
          max={node.maximum}
          integer
          disabled={disabled}
          onChange={onChange}
          aria-invalid={invalid}
        />
      );
    case "number":
      return (
        <FieldNumber
          id={id}
          value={typeof value === "number" ? value : ""}
          step={node.step ?? 0.01}
          min={node.minimum}
          max={node.maximum}
          disabled={disabled}
          onChange={onChange}
          aria-invalid={invalid}
        />
      );
    case "array":
      if (node.itemType === "string") {
        const arr = Array.isArray(value)
          ? value.filter((v): v is string => typeof v === "string")
          : [];
        return (
          <FieldStringArray
            id={id}
            value={arr}
            onChange={onChange}
            disabled={disabled}
          />
        );
      }
      return (
        <FieldJson
          id={id}
          value={value}
          onChange={onChange}
          disabled={disabled}
          aria-invalid={invalid}
        />
      );
    case "object":
      return (
        <FieldJson
          id={id}
          value={value}
          onChange={onChange}
          disabled={disabled}
          aria-invalid={invalid}
        />
      );
    case "string":
    default:
      return (
        <FieldString
          id={id}
          value={typeof value === "string" ? value : ""}
          onChange={onChange}
          disabled={disabled}
          aria-invalid={invalid}
        />
      );
  }
}
