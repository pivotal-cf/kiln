### <a id='{{ .Version }}'></a> {{ .Version }}

{{ if not .ReleaseDate.IsZero }}**Release Date:** {{ .ReleaseDate.Format "01/02/2006" }}{{ end }}

{{ define "component" }}
    <tr><td>{{ .Name }}</td><td>{{ .Version }}</td><td></td></tr>
{{- end -}}

{{ define "component-legacy" }}
    <tr><td>{{ .Name }}</td><td>{{ .Version }}</td></tr>
{{- end -}}

{{ define "component-with-notes" }}
    <tr><td>{{ .Name }}</td><td>{{ .Version }}</td>
      <td>{{- range .Releases}}
        {{- if ne (trim .GetBody) "" }}
        <details>
          <summary>{{ .GetTagName }}</summary>
          <pre style="max-width: 30em">
  {{ trim .GetBody | removeEmptyLines | indent 2 | trim}}
          </pre>
        </details>
        {{- end }}
      {{- end}}
      </td>
    </tr>
{{- end -}}

{{range .Issues -}}
  * {{.GetTitle}}
{{ end -}}
{{range .TrainstatNotes -}}
  {{.}}
{{ end -}}
{{range .Bumps -}}
  * Bump {{ .Name }} to version `{{ .ToVersion }}`
{{ end }}
<table border="1" class="nice">
  <thead>
    <tr>
      <th>Component</th>
      <th>Version</th>{{if .HasComponentReleases}}
      <th>Release Notes</th>{{end}}
    </tr>
  </thead>
  <tbody>
  {{- if .Stemcell.OS }}
    <tr><td>{{ .Stemcell.OS }} stemcell</td><td>{{ .Stemcell.Version }}</td>{{- if $.HasComponentReleases -}}<td></td>{{ end }}</tr>
  {{- end -}}
  {{- range .Components }}
    {{- if not $.HasComponentReleases -}}
       {{template "component-legacy" .}}
    {{- else if .HasReleaseNotes -}}
      {{template "component-with-notes" .}}
    {{- else -}}
      {{template "component" .}}
    {{- end -}}
  {{- end }}
  </tbody>
</table>


{{/* most releaes notes have 4 new lines between them */}}
