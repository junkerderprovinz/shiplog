<?php
/* Checks the Docker Hub credentials typed on the settings page, before Apply,
 * the same way the resolver uses them: a token request with Basic auth.
 * Returns {"ok":bool,"message":string}. */

header('Content-Type: application/json');

$user  = isset($_GET['user'])  ? trim($_GET['user'])  : '';
$token = isset($_GET['token']) ? trim($_GET['token']) : '';

if ($user === '' || $token === '') {
    echo json_encode(['ok' => false, 'message' => 'Enter the Docker Hub username and token first.']);
    exit;
}

$url = 'https://auth.docker.io/token?service=registry.docker.io&scope='
     . rawurlencode('repository:library/hello-world:pull');

$ch = curl_init($url);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 4,
    CURLOPT_TIMEOUT        => 10,
    CURLOPT_HTTPAUTH       => CURLAUTH_BASIC,
    CURLOPT_USERPWD        => $user . ':' . $token,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    echo json_encode(['ok' => false, 'message' => 'Cannot reach Docker Hub auth.']);
    exit;
}
if ($code === 401 || $code === 403) {
    echo json_encode(['ok' => false, 'message' => 'Docker Hub rejected these credentials (HTTP ' . $code . ').']);
    exit;
}
if ($code !== 200) {
    echo json_encode(['ok' => false, 'message' => 'Docker Hub auth returned HTTP ' . $code . '.']);
    exit;
}

$data = json_decode($body, true);
if (is_array($data) && (!empty($data['token']) || !empty($data['access_token']))) {
    echo json_encode(['ok' => true, 'message' => 'Credentials accepted by Docker Hub.']);
} else {
    echo json_encode(['ok' => false, 'message' => 'Unexpected response from Docker Hub auth.']);
}
