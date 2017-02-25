<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <title>{{.VisibleTitle}}</title>
    <link rel="stylesheet" type="text/css" href="{{.StaticRoot}}/style.css" />
    <link rel="stylesheet" type="text/css" href="/static/wiki.css" />
    {{with .PageCSS}}
    <style type="text/css">
    {{.}}
    </style>
    {{end}}
    <script type="text/javascript" src="{{.StaticRoot}}/retina.min.js"></script>
</head>

<body onload="retinajs()">
<div id="container">

    <div id="header">
        <ul id="navigation">
            <li><a href="/">Main page</a></li>
        </ul>
        <a href="/">
            {{with .WikiLogo}}
            <img src="{{.}}" alt="Wiki" />
            {{else}}
            <h1>{{.WikiTitle}}</h1>
            {{end}}
        </a>
    </div>

    <div id="content">