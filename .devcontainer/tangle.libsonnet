{
  name: "fullstack",
  forwardPorts: [8081],
  customizations+: {
    vscode+: {
      extensions+: [
        "github.vscode-github-actions",
        "ritwickdey.LiveServer",
        "svelte.svelte-vscode",
        "ms-vscode.vscode-typescript-next",
      ],
    },
  },
  postCreateCommand+: {
    "tangle-post-install": "bash ./.devcontainer/post_install.sh",
  },
}
