// A stand-in for the Wails bridge, so the built page can be opened in a
// normal browser: every Go method the page calls is a promise here, and
// every call is appended to window.calls for the test to read. `emit`
// stands in for the `shown` event ("window.emit('links')").
window.calls = [];
window.favs = [];

window.runtime = {
    EventsOn: (name, callback) => {
        window.emit = callback;
    },
};

window.go = {main: {App: {
    Apps: async () => [
        {id: 'cronometro', name: 'Cronómetro', description: 'Cuenta atrás', bundle: 'cronometro.app', category: 'utilidades', path: '/x'},
    ],
    AppCategories: async () => [{id: 'utilidades', name: 'Utilidades'}],
    Links: async () => [
        {id: 'mdn', name: 'MDN', description: 'Referencia web', url: 'https://developer.mozilla.org', category: 'documentacion', host: 'developer.mozilla.org'},
        {id: 'gitea', name: 'Gitea', description: 'Repos', url: 'https://repos.example.com', category: 'ecosistema', host: 'repos.example.com'},
    ],
    LinkCategories: async () => [{id: 'ecosistema', name: 'Mi ecosistema'}, {id: 'documentacion', name: 'Documentación'}],
    Usage: async () => ({opens: {'links:gitea': 2}, favorites: window.favs}),
    Launch: async (id) => { window.calls.push(`Launch:${id}`); },
    OpenLink: async (id) => { window.calls.push(`OpenLink:${id}`); },
    Hide: async () => { window.calls.push('Hide'); },
    TabChanged: async (tab) => { window.calls.push(`TabChanged:${tab}`); },
    Debug: async (message) => { window.calls.push(`Debug:${message}`); },
    ToggleFavorite: async (key) => {
        window.calls.push(`ToggleFavorite:${key}`);
        const index = window.favs.indexOf(key);
        if (index >= 0) {
            window.favs.splice(index, 1);
            return false;
        }
        window.favs.push(key);
        return true;
    },
}}};

window.addEventListener('error', (event) => {
    window.calls.push(`ERROR:${event.message}`);
});
