package proxy

var htmlContent = `
<!DOCTYPE html>
<html>
<head>
    <title>Access Denied</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        h1 { color: #d9534f; }
    </style>
</head>
<body>
    <h1>403 Forbidden</h1>
    <p>Access to ${domain} has been restricted by the administrator.</p>
    <p>If you believe this is an error, please contact support.</p>
</body>
</html>
    `
