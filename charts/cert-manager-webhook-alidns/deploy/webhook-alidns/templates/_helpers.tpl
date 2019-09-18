{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "webhook-alidns.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "webhook-alidns.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "webhook-alidns.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "webhook-alidns.selfSignedIssuer" -}}
{{ printf "%s-selfsign" (include "webhook-alidns.fullname" .) }}
{{- end -}}

{{- define "webhook-alidns.rootCAIssuer" -}}
{{ printf "%s-ca" (include "webhook-alidns.fullname" .) }}
{{- end -}}

{{- define "webhook-alidns.rootCACertificate" -}}
{{ printf "%s-ca" (include "webhook-alidns.fullname" .) }}
{{- end -}}

{{- define "webhook-alidns.servingCertificate" -}}
{{ printf "%s-webhook-tls" (include "webhook-alidns.fullname" .) }}
{{- end -}}
