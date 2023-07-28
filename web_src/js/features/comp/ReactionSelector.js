import $ from 'jquery';

const {csrfToken} = window.config;

export function initCompReactionSelector($parent) {
  $parent.find(`.select-reaction .item.reaction, .comment-reaction-button`).on('click', async function (e) {
    e.preventDefault();

    if ($(this).hasClass('disabled')) return;

    const actionUrl = $(this).closest('[data-action-url]').attr('data-action-url');
    const reactionContent = $(this).attr('data-reaction-content');
    const hasReacted = $(this).closest('.ui.segment.reactions').find(`a[data-reaction-content="${reactionContent}"]`).attr('data-has-reacted') === 'true';

    const res = await fetch(`${actionUrl}/${hasReacted ? 'unreact' : 'react'}`, {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({
        _csrf: csrfToken,
        content: reactionContent,
      }),
    });

    const data = await res.json();
    if (data && (data.html || data.empty)) {
      const content = $(this).closest('.content');
      let react = content.find('.segment.reactions');
      if ((!data.empty || data.html === '') && react.length > 0) {
        react.remove();
      }
      if (!data.empty) {
        const attachments = content.find('.segment.bottom:first');
        react = $(data.html);
        if (attachments.length > 0) {
          react.insertBefore(attachments);
        } else {
          react.appendTo(content);
        }
        react.find('.dropdown').dropdown();
        initCompReactionSelector(react);
      }
    }
  });
}
